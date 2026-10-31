package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// Assigning a profile to an account is the write that makes every other page on this
// server matter: a profile with no account under it enforces nothing, and an account with
// no profile is never locked out. It is also the only write whose absence was invisible —
// the profiles page could be built, tested and merged while the daemon still had no way to
// be told which child the rules apply to.
//
// One route carries both halves of the row's state because they are one decision. "Manage
// this account with this profile, on or off" is a single intent, and splitting it into two
// forms would let the page offer a combination it refuses to save: enabling an account with
// no profile is not a state config.Validate accepts (a user whose profileID names no
// existing profile is refused), so a separate enable-only form would have to invent a
// profile id to satisfy a check that is only correct for the pair.

// The refusal sentinels are the ones the override path already declared — errUnknownAccount
// and errUnknownProfile. Re-declaring a second pair for this route would put two errors with
// the same meaning in one package, and the next handler would pick whichever one it happened
// to see first.
//
// errNoProfileSelected is the exception, and the exception is deliberate: it is the empty
// profile field arriving on an account that has no entry to remove. It is not the same request
// as "stop managing" and must not borrow that message — the shape is what a machine with no
// profiles produces when somebody presses Save meaning to start managing, and telling them
// there is nothing to stop hides the thing actually missing. (#192) Raised and translated only
// here, so it lives on the route that owns it rather than beside errUnknownAccount, which
// several routes share.
var errNoProfileSelected = errors.New("no profile was selected")

// handleAssignProfile is POST /users/{name}/profile.
//
// An empty profile field means "stop managing this account", and that goes through
// config.RemoveUser rather than UpsertUser with a blank id. Two reasons, and the second is
// the one that would bite: RemoveUser drops the account's override in the same move, on the
// reasoning recorded there that a grant left behind would be inherited by whoever next uses
// that account. Writing User{ProfileID: ""} instead would not merely be refused by
// Validate — it would be refused *after* the caller believed the account was unmanaged,
// which is the difference between a refusal and a half-done change.
func (s *Server) handleAssignProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.pageRefuse(w, r, http.StatusBadRequest, "no account was named")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.pageRefuse(w, r, http.StatusBadRequest, "the form could not be read")
		return
	}
	profileID := strings.TrimSpace(r.FormValue("profile"))
	// A checkbox sends nothing when it is clear, so absence is the off position. That is
	// the same reading the rule form gives its "allDay" box, and it is the direction that
	// fails safe: a form that lost its checkbox stops managing an account rather than
	// silently starting to.
	enabled := r.FormValue("enabled") != ""

	// The account has to be one this page would have offered. Without this, POSTing a
	// name that is neither a local account nor already configured writes a document
	// entry for nobody — and Validate cannot catch it, because "username is non-empty
	// and unique" is true of every name a person could type.
	//
	// The listing is read here rather than trusted from the page because the page is a
	// snapshot: an account deleted since it was rendered is exactly the row whose
	// assignment would do nothing.
	known, err := s.knownAccount(name)
	if err != nil {
		// A failed listing is the server's failure, not the operator's wrong request.
		// Turning it into "no such account" would be the same lie as a failed account
		// read answering "no such account" on the audit page: the operator is told the
		// thing they typed does not exist when the truth is that nothing could be asked.
		s.pageError(w, r, "the account list could not be read", err)
		return
	}
	if !known {
		s.pageRefuse(w, r, http.StatusNotFound,
			"no account on this machine is called "+name+", and it is not already managed")
		return
	}

	assigned := ""
	if err := s.mutateConfig(func(c *config.Config) error {
		if profileID == "" {
			// An empty field means two different things and only one of them is a removal.
			// The dropdown offers "not managed" as its empty choice, so this is also what
			// arrives when there were no other choices to render. The document decides which
			// case this is: with no profiles at all, "nothing to stop managing" describes the
			// opposite of what the operator meant -- they pressed Save to start managing and
			// had no way to. With profiles available, an empty field on an unmanaged account
			// really is a request to remove an entry that is not there. Checked inside the
			// mutation because that is the document the write would land in. (#192)
			if len(c.Profiles) == 0 && !knownAccount(c, name) {
				return fmt.Errorf("%w: %s", errNoProfileSelected, name)
			}
			if err := c.RemoveUser(name); err != nil {
				return fmt.Errorf("%w: %s", errUnknownAccount, err)
			}
			return nil
		}
		if _, ok := findProfile(c, profileID); !ok {
			return errUnknownProfile
		}
		// UpsertUser writes into a fresh array, so a snapshot taken before this request
		// keeps the assignment it was rendered from. Nothing here touches c.Users in
		// place.
		c.UpsertUser(config.User{Username: name, ProfileID: profileID, Enabled: enabled})
		assigned = profileID
		return nil
	}); err != nil {
		switch {
		case errors.Is(err, errUnknownProfile):
			s.pageRefuse(w, r, http.StatusNotFound, "no profile on this machine has that id")
		case errors.Is(err, errNoProfileSelected):
			// Names the missing thing and where to make it. The accounts page refuses to
			// render a dropdown with nothing in it (#192), so reaching this from the browser
			// means the document lost its last profile between the page and the POST -- the
			// message has to stand on its own either way.
			s.pageRefuse(w, r, http.StatusBadRequest,
				"no profile is selected, and a profile is what manages an account. Create one on the profiles page first.")
		case errors.Is(err, errUnknownAccount):
			s.pageRefuse(w, r, http.StatusNotFound,
				"that account is not managed, so there is nothing to stop managing")
		default:
			s.mutateRefused(w, r, "the assignment could not be saved", err)
		}
		return
	}

	if assigned == "" {
		s.audit("user.unassigned", fmt.Sprintf("account %s is no longer managed by %s",
			name, sessionName(s, r)))
	} else {
		s.audit("user.assigned", fmt.Sprintf("account %s profile %s%s by %s", name, assigned,
			enabledWord(enabled), sessionName(s, r)))
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// enabledWord keeps the audit line a complete sentence about what the row now says. The
// enabled flag is the difference between a profile that fires tonight and one that does
// not, and an audit trail that records the assignment but not the switch is a trail that
// has to be read next to the config to mean anything.
func enabledWord(enabled bool) string {
	if enabled {
		return " enabled"
	}
	return " switched off"
}

// knownAccount reports whether the name is either a discovered local account or already
// in the configuration.
//
// The second half is what keeps the union honest: config names an account the password
// file no longer has, the users page renders that row as stale, and the operator has to be
// able to change it — including stopping it. Refusing on OnMachine alone would strand the
// one row whose cleanup this page exists to offer.
func (s *Server) knownAccount(name string) (bool, error) {
	cfg, err := s.readConfig()
	if err != nil {
		return false, err
	}
	for _, u := range cfg.Users {
		if u.Username == name {
			return true, nil
		}
	}
	listing, err := s.readAccounts()
	if err != nil {
		return false, err
	}
	for _, a := range listing.Accounts {
		if a.Username == name {
			return true, nil
		}
	}
	return false, nil
}
