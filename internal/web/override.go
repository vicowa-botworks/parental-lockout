package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/schedule"
)

// The override routes: a parent letting one child out for a while.
//
// An override is the only mutation on this branch, and it is the one that most needs the
// discipline below, because it is the mutation a child has an incentive to try to cause and
// the one a parent needs to be able to prove afterwards. Every refusal therefore says which
// rule refused it, and every acceptance writes an audit line naming who granted it.

// overrideMinutes is the set of durations the UI may ask for.
//
// A free-form number would be a footgun with no guardrail behind it: "override for 5" and
// "override for 5 hours" are both one keystroke, and the second one silently unlocks a
// child for a day and a half. The ticket names 15/30/60, and the buttons on the page are
// those three, so anything else is a hand-crafted request rather than a use case.
var overrideMinutes = []int{15, 30, 60}

// maxOverrideReason bounds the free-text field.
//
// The other two inputs to this request are bounded -- the minutes come from a set, the
// scope from a checkbox -- and an unbounded one is the odd shape out. A reason is stored in
// the config document, written to the audit line, and rendered on the dashboard row, so a
// megabyte of it is a megabyte in all three. The writer is an authenticated admin, so this
// is not an attack surface; it is the same guardrail the rest of the form has, and a
// refusal that names the limit is friendlier than a dashboard row nobody can read.
const maxOverrideReason = 500

// overrideReason is the one place the reason is judged, so the web form and `plock unlock`
// cannot answer the same field differently.
//
// Blank is refused, and so is whitespace -- `--reason " "` from a shell and a form field
// the operator typed a space into both produce a stored reason that renders as nothing.
// The audit line then ends in a colon and the trail says the child was let out with no
// sentence after it, which is the exact entry the trail exists to settle. An override is
// the one config entry that REMOVES enforcement.
func overrideReason(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("an override needs a reason; it goes in the audit trail")
	}
	if len(trimmed) > maxOverrideReason {
		return "", fmt.Errorf("the reason is %d characters; keep it under %d so it stays readable on the dashboard and in the audit trail",
			len(trimmed), maxOverrideReason)
	}
	return trimmed, nil
}

// handleOverride grants or clears an override.
//
// The username comes from the path, so it is attacker-shaped text. It is never used to build
// a path or a command -- it is compared against the accounts the daemon already knows -- and
// an unknown name is refused rather than written down, because an override for an account
// that does not exist is a record that says somebody was let out and no such person exists.
func (s *Server) handleOverride(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.pageRefuse(w, r, http.StatusBadRequest, "no account was named")
		return
	}

	// The reason is required, and required means the request is refused without it rather
	// than defaulted. An override with no reason is the entry nobody can reconstruct at
	// the kitchen table: the audit line says the child was let out and offers nothing about
	// why, which is exactly the situation the trail exists to settle.
	reason, err := overrideReason(r.FormValue("reason"))
	if err != nil {
		s.pageRefuse(w, r, http.StatusUnprocessableEntity, err.Error())
		return
	}

	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}
	if !knownAccount(cfg, name) {
		s.pageRefuse(w, r, http.StatusNotFound,
			"that account is not managed on this machine")
		return
	}

	until, err := s.overrideUntil(cfg, name, r)
	if err != nil {
		s.pageRefuse(w, r, http.StatusUnprocessableEntity, err.Error())
		return
	}

	by := "unknown"
	if sess, ok := s.sessionFrom(r); ok {
		by = sess.Username
	}

	if err := s.mutateConfig(func(c *config.Config) error {
		// Re-checked inside the mutation. The cfg read above is outside the write lock, so
		// a concurrent request could have removed the account between the two. The check is
		// cheap and the alternative is writing an override for a name that stopped existing.
		if !knownAccount(c, name) {
			return errUnknownAccount
		}
		c.AddOverride(config.Override{Username: name, Until: until, Reason: reason, By: by})
		return nil
	}); err != nil {
		if errors.Is(err, errUnknownAccount) {
			s.pageRefuse(w, r, http.StatusNotFound,
				"that account is not managed on this machine")
			return
		}
		s.mutateRefused(w, r, "the override could not be saved", err)
		return
	}

	s.audit("override.granted", fmt.Sprintf("%s until %s by %s: %s", name,
		until.UTC().Format(time.RFC3339), by, reason))
	s.logf("web: %s override granted for %s until %s", requestID(r), name, until.Format(time.RFC3339))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleOverrideClear removes an active override.
//
// Clearing is not the same as the override expiring: the child goes back under enforcement
// immediately, so this is a tightening and it is audited like one.
func (s *Server) handleOverrideClear(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.pageRefuse(w, r, http.StatusBadRequest, "no account was named")
		return
	}

	removed := false
	if err := s.mutateConfig(func(c *config.Config) error {
		removed = c.RemoveOverride(name)
		return nil
	}); err != nil {
		s.mutateRefused(w, r, "the override could not be cleared", err)
		return
	}
	if !removed {
		// Not an error, and not silence either: the page said "clear the override" and the
		// honest answer is that there was none, which the operator needs to hear because it
		// means the child is already back under enforcement.
		s.pageRefuse(w, r, http.StatusNotFound, "that account has no override to clear")
		return
	}

	by := "unknown"
	if sess, ok := s.sessionFrom(r); ok {
		by = sess.Username
	}
	s.audit("override.cleared", fmt.Sprintf("%s by %s", name, by))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// errUnknownAccount distinguishes "the account vanished under the write lock" from a save
// failure, so the handler can answer 404 rather than 503.
var errUnknownAccount = errors.New("the account is no longer managed")

// overrideUntil turns the form into an instant.
//
// "Until the current window ends" is resolved here rather than stored as a sentinel, because
// a stored sentinel would have to be re-resolved on every tick by whoever enforces it, and
// the window can change underneath it -- a parent editing the rule would silently extend or
// shorten an override that was supposed to be bounded. A concrete instant cannot drift.
func (s *Server) overrideUntil(cfg *config.Config, name string, r *http.Request) (time.Time, error) {
	now := s.now()

	if r.FormValue("scope") == "window" {
		end, ok := windowEnd(cfg, name, now)
		if !ok {
			return time.Time{}, errors.New("that account is not inside a window right now, " +
				"so there is no window end to unlock until")
		}
		return end, nil
	}

	raw := r.FormValue("minutes")
	m, err := strconv.Atoi(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("the override length %q is not a number of minutes", raw)
	}
	for _, ok := range overrideMinutes {
		if m == ok {
			return now.Add(time.Duration(m) * time.Minute), nil
		}
	}
	return time.Time{}, fmt.Errorf("an override may be %s minutes, not %d",
		listMinutes(), m)
}

func listMinutes() string {
	out := ""
	for i, m := range overrideMinutes {
		if i > 0 {
			out += ", "
		}
		out += strconv.Itoa(m)
	}
	return out
}

// knownAccount is the set of names an override may be granted for: an account the daemon
// manages in the config.
//
// Deliberately not the machine's whole account list. A human account with no config entry is
// not under enforcement, so an override for it would be a record of letting out somebody who
// was never held -- and the audit trail is the artifact a parent reads to find out what
// happened.
func knownAccount(c *config.Config, name string) bool {
	for i := range c.Users {
		if c.Users[i].Username == name {
			return true
		}
	}
	return false
}

// windowEnd finds when the account's current lockout window closes, through the same
// evaluator the enforcement loop uses so the two cannot disagree about when a child is free.
func windowEnd(c *config.Config, name string, now time.Time) (time.Time, bool) {
	var profileID string
	for i := range c.Users {
		if c.Users[i].Username == name {
			profileID = c.Users[i].ProfileID
			break
		}
	}
	if profileID == "" {
		return time.Time{}, false
	}
	for i := range c.Profiles {
		if c.Profiles[i].ID != profileID {
			continue
		}
		v := schedule.Evaluate(&c.Profiles[i], now)
		if !v.Active {
			return time.Time{}, false
		}
		return v.EndsAt, true
	}
	return time.Time{}, false
}
