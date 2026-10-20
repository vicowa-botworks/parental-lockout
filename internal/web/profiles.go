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

// The profiles page: what a lockout profile enforces, which children share it, and the four
// writes a parent makes against it -- add a rule, remove the profile, and change the two
// labels on it: the name and the timezone.
//
// Both writes go through mutateConfig, so both are validated before anything is saved. That
// is load-bearing here in a way it was not for the override path: a hand-typed rule can be a
// window that enforces nothing, and config.Validate is the one place that knows every shape
// it refuses (an empty window, a day list naming no day, a clock that is not HH:MM). A
// handler that checked only the fields it thought about would be a second, weaker copy of
// that list, drifting from it one field at a time.

// profilesPage is what the template renders.
type profilesPage struct {
	Profiles []profileRow
	// Days is config.DayNames in its canonical order, so the form's checkboxes are the
	// same seven the evaluator accepts rather than a list this package re-typed.
	Days []string
	// Manage is the master switch: with enforcement off, adding a rule changes a
	// document nothing acts on, and the page has to say so rather than look idle.
	Manage bool

	// CSRF is filled by render through csrfCarrier, never by the handler.
	CSRF string
}

// csrfCarrier itself is declared once, in pages.go, next to the render hook that uses it.

func (p *profilesPage) setCSRF(token string) { p.CSRF = token }

type profileRow struct {
	ID       string
	Name     string
	Timezone string
	Rules    []ruleRow
	// AssignedTo is every account sharing this profile. It is on the row because it is
	// the answer to "what breaks if I delete this", and the delete refuses on exactly
	// that list.
	AssignedTo []string
	// Complaint is the evaluator's own degraded report for the profile -- an unknown
	// timezone falling back to a default, a rule that can never fire. It is the
	// evaluator's sentence rather than one written here, so the page cannot present a
	// profile as healthy that the enforcement loop considers broken.
	Complaint string
}

type ruleRow struct {
	ID      string
	Name    string
	Days    string
	Window  string
	Enabled bool
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}
	// A pointer, so render can see csrfCarrier: the interface is implemented on
	// *profilesPage, and a value handed over as any would not satisfy it -- the page
	// would render every form with an empty token and every submit would be refused.
	page := s.profilesView(cfg)
	s.render(w, r, "profiles", &page)
}

// profilesView builds the page from the document. It takes the config rather than reading it
// so the tests can hand it a document and assert on the rendering without a request.
func (s *Server) profilesView(cfg *config.Config) profilesPage {
	page := profilesPage{Days: config.DayNames, Manage: cfg.Settings.Enforcement}

	for _, p := range cfg.Profiles {
		row := profileRow{
			ID: p.ID, Name: p.Name, Timezone: p.Timezone,
			Rules: make([]ruleRow, 0, len(p.Rules)),
		}
		for _, u := range cfg.Users {
			if u.ProfileID == p.ID {
				row.AssignedTo = append(row.AssignedTo, u.Username)
			}
		}
		for _, rl := range p.Rules {
			row.Rules = append(row.Rules, ruleRow{
				ID:      rl.ID,
				Name:    rl.Name,
				Days:    strings.Join(rl.Days, ", "),
				Window:  ruleWindow(rl),
				Enabled: rl.Enabled,
			})
		}
		// ProfileErrors is the evaluator's own list of rules it will skip and why, so
		// the page cannot present a profile as healthy that the enforcement loop
		// considers broken. It is a function rather than a Verdict field because the
		// rules it names are skipped before any window is evaluated.
		if errs := schedule.ProfileErrors(&p); len(errs) > 0 {
			msgs := make([]string, len(errs))
			for i, e := range errs {
				msgs[i] = e.Error()
			}
			row.Complaint = strings.Join(msgs, "; ")
		}
		page.Profiles = append(page.Profiles, row)
	}
	return page
}

// ruleWindow renders the clock pair, or the words "all day" for a rule whose ends are not
// consulted at all. Showing 00:00-23:59 next to an all-day rule would be actively misleading:
// that window is a minute short of the rule the checkbox means.
func ruleWindow(rl config.Rule) string {
	if rl.AllDay {
		return "all day"
	}
	start, end := rl.Start, rl.End
	if start == "" {
		start = "--:--"
	}
	if end == "" {
		end = "--:--"
	}
	return start + " - " + end
}

// handleAddRule appends a rule to a profile.
//
// The rule id is derived rather than typed. An id the operator supplies is a collision
// waiting to happen -- config.Validate refuses a profile that uses one id twice -- and the
// form has no use for a second name besides the one already in it.
func (s *Server) handleAddRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.pageRefuse(w, r, http.StatusBadRequest, "no profile was named")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.pageRefuse(w, r, http.StatusBadRequest, "the form could not be read")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			"a rule needs a name; it is what the audit trail and this page both point at")
		return
	}

	allDay := r.FormValue("allDay") != ""
	days := r.Form["days"]
	if len(days) == 0 {
		// Refused here rather than left to Validate, which would report it as a rule
		// that names no day. The form knows a box was never ticked; that is a better
		// sentence than the document-level one.
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			"pick at least one day, or the rule would enforce nothing on any day")
		return
	}

	rule := config.Rule{
		Name:    name,
		Days:    days,
		AllDay:  allDay,
		Enabled: true,
	}
	if !allDay {
		rule.Start = strings.TrimSpace(r.FormValue("start"))
		rule.End = strings.TrimSpace(r.FormValue("end"))
	}

	created := ""
	if err := s.mutateConfig(func(c *config.Config) error {
		p, ok := findProfile(c, id)
		if !ok {
			return errUnknownProfile
		}
		rule.ID = freshRuleID(p.Rules, name)
		created = rule.ID
		// A fresh slice, then UpsertProfile: neither the existing element nor its
		// backing array is written through. See findProfile.
		p.Rules = append(append([]config.Rule(nil), p.Rules...), rule)
		c.UpsertProfile(p)
		return nil
	}); err != nil {
		if errors.Is(err, errUnknownProfile) {
			s.pageRefuse(w, r, http.StatusNotFound, "no profile on this machine has that id")
			return
		}
		s.mutateRefused(w, r, "the rule could not be saved", err)
		return
	}

	s.audit("rule.added", fmt.Sprintf("profile %s rule %s (%s) by %s", id, created,
		ruleWindow(rule), sessionName(s, r)))
	http.Redirect(w, r, "/profiles", http.StatusSeeOther)
}

// handleDeleteProfile removes a profile.
//
// The refusal is the interesting half. config.DeleteProfile refuses while any account still
// points at the profile, and names every one of them, because dropping a profile out from
// under a child leaves a dangling profileID that Validate rejects and the enforcement loop
// would have to guess a meaning for.
func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.pageRefuse(w, r, http.StatusBadRequest, "no profile was named")
		return
	}

	if err := s.mutateConfig(func(c *config.Config) error {
		return c.DeleteProfile(id)
	}); err != nil {
		switch {
		case errors.Is(err, config.ErrProfileInUse):
			// The message already names every blocking account, which is the whole answer, so
			// it goes through as a refusal rather than being paraphrased into vagueness.
			s.pageRefuse(w, r, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, config.ErrNoSuchProfile):
			// Its own sentence rather than err.Error(): the id came from the URL, and reflecting
			// request text back into a response body is a habit worth not having even when the
			// response is plain text and nothing can execute.
			s.pageRefuse(w, r, http.StatusNotFound, "there is no profile with that id")
		default:
			// Everything left is the seam failing: config.Load could not read the file, or
			// Save could not write it. That is "the daemon could not do the work", which is a
			// 503 and retryable, not a statement that the profile does not exist -- and the raw
			// error (a parse failure, a permission denial) is logged, not echoed.
			s.mutateRefused(w, r, "the profile could not be deleted", err)
		}
		return
	}

	s.audit("profile.deleted", fmt.Sprintf("profile %s by %s", id, sessionName(s, r)))
	http.Redirect(w, r, "/profiles", http.StatusSeeOther)
}

var errUnknownProfile = errors.New("no such profile")

// errProfileNameTaken marks a rename refused because a different profile already answers to
// that name. It is a sentinel rather than a fmt.Errorf so the handler can answer 422 with its
// own sentence instead of paraphrasing one built from the colliding name -- see the comment on
// handleProfileRename.
var errProfileNameTaken = errors.New("that profile name is already taken")

// handleProfileTimezone changes the zone a profile's windows are read in.
//
// This is the one field on the page that is not cosmetic. Every window in the profile is
// resolved against this string, so a wrong zone does not mislabel anything -- it enforces the
// right clock at the wrong hours, and the dashboard keeps reporting lockouts computed in the
// wrong zone as though they were the ones the operator set.
//
// The zone is checked here with the same call the evaluator makes, before anything is written.
// config.Validate refuses an unusable zone too, and mutateConfig runs it on the way out, so
// the document cannot end up broken either way; what the early check buys is the sentence.
// Validate's is a document-level "profiles[0] (school).timezone: ... is not a usable
// timezone", which is the right thing for a hand-edited file and the wrong thing to answer a
// form field with.
func (s *Server) handleProfileTimezone(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.pageRefuse(w, r, http.StatusBadRequest, "no profile was named")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.pageRefuse(w, r, http.StatusBadRequest, "the form could not be read")
		return
	}

	zone := strings.TrimSpace(r.FormValue("timezone"))
	if zone == "" {
		// Refused rather than saved as blank, because blank is not "unset" here:
		// time.LoadLocation("") answers UTC. A cleared field would move every window in
		// the profile to UTC while the page still looked like a profile that had simply
		// not been filled in. Validate says the same thing at document level; this is the
		// form-level sentence, and the days checkbox on the rule form set the precedent.
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			"a profile needs a timezone; a blank one is read as UTC, which would move every "+
				"window in it without anyone choosing that")
		return
	}
	if _, err := time.LoadLocation(zone); err != nil {
		// The typed value is deliberately not echoed back. It came from the request, and
		// reflecting request text into a response body is a habit this package does not
		// have even when the response is plain text (see handleDeleteProfile).
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			"that is not a timezone this machine can load; use a zone name such as "+
				"Europe/Paris, from the IANA zone list")
		return
	}
	if zone == "Local" {
		// `Local` loads, so the check above lets it through and Validate agrees it is a zone.
		// It is the one name whose meaning is "whatever this machine says right now": every
		// window in the profile would move whenever the host's zone moved, and the evaluator's
		// degraded-report path stays silent because the name resolves. A field that promises
		// "every window below is read in this zone" cannot hold the one value that answers to
		// no zone at all -- and the machine's own zone is set by the distro installer, not by
		// the parent reading this page.
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			"Local is not a timezone for a profile; it follows whatever the machine's clock is "+
				"set to, so the hours would move on their own. Pick the zone the household "+
				"lives in, such as Europe/Paris.")
		return
	}

	previous := ""
	if err := s.mutateConfig(func(c *config.Config) error {
		p, ok := findProfile(c, id)
		if !ok {
			return errUnknownProfile
		}
		previous = p.Timezone
		p.Timezone = zone
		// The copy from findProfile, handed back through UpsertProfile: the element and its
		// backing array are never written through. See findProfile.
		c.UpsertProfile(p)
		return nil
	}); err != nil {
		if errors.Is(err, errUnknownProfile) {
			s.pageRefuse(w, r, http.StatusNotFound, "no profile on this machine has that id")
			return
		}
		s.mutateRefused(w, r, "the timezone could not be saved", err)
		return
	}

	// The old zone goes in the line, not just the new one. This is the edit that moves every
	// window an account has, and "what were the hours before?" is the first question anybody
	// asking about a lockout is going to have.
	s.audit("profile.rezoned", fmt.Sprintf("profile %s %s -> %s by %s",
		id, previous, zone, sessionName(s, r)))
	http.Redirect(w, r, "/profiles", http.StatusSeeOther)
}

// handleProfileRename relabels a profile.
//
// Only the name moves. The id is what every account points at and what the rule ids are
// derived from, so a rename that touched either would renumber rules or orphan children --
// TestARenameLeavesTheRuleIDsAndOrderAlone is the invariant, and it is why this handler sets
// one field on the copy rather than rebuilding the profile.
//
// A duplicate name is refused. config.Validate does not check names, only ids, and the page is
// what makes a duplicate cost something: it heads each profile with its name and falls back to
// the id only when the name is empty, so two profiles called "Kids" render two identical
// headings, two identical table captions, and two identical delete confirmations for different
// documents.
//
// This is the web path's guarantee, and its ceiling is worth stating: `plock profile add
// --copy-from` carries the source's name onto the new profile and `plock profile edit --name`
// sets one with no collision check, so the CLI can still create the pair. Filed rather than
// fixed here -- the right home for the rule is wherever both paths can be made to share it.
func (s *Server) handleProfileRename(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.pageRefuse(w, r, http.StatusBadRequest, "no profile was named")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.pageRefuse(w, r, http.StatusBadRequest, "the form could not be read")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			"a profile needs a name; leaving the field blank would not clear it, it would "+
				"just make the page fall back to the id")
		return
	}

	previous := ""
	if err := s.mutateConfig(func(c *config.Config) error {
		p, ok := findProfile(c, id)
		if !ok {
			return errUnknownProfile
		}
		for _, other := range c.Profiles {
			if other.ID == p.ID {
				continue
			}
			// The id counts as a label too, because the page prints it as one: the heading is
			// "name, or the id when the name is empty", and Validate requires an id but never a
			// name. So a profile listed under its id is ambiguous with a new profile named after
			// that id in exactly the way two identical names are, and checking names alone let
			// the collision back in through the fallback that exists to cover a missing name.
			if strings.TrimSpace(other.Name) == name || other.ID == name {
				return errProfileNameTaken
			}
		}
		previous = p.Name
		p.Name = name
		c.UpsertProfile(p)
		return nil
	}); err != nil {
		switch {
		case errors.Is(err, errUnknownProfile):
			s.pageRefuse(w, r, http.StatusNotFound, "no profile on this machine has that id")
		case errors.Is(err, errProfileNameTaken):
			// Its own sentence rather than err.Error(), which carries no name to leak and
			// would read as a system fault rather than a collision the operator can fix.
			s.pageRefuse(w, r, http.StatusUnprocessableEntity,
				"another profile on this machine already has that name, and this page lists "+
					"profiles by name -- two alike could not be told apart")
		default:
			s.mutateRefused(w, r, "the profile could not be renamed", err)
		}
		return
	}

	s.audit("profile.renamed", fmt.Sprintf("profile %s %q -> %q by %s",
		id, previous, name, sessionName(s, r)))
	http.Redirect(w, r, "/profiles", http.StatusSeeOther)
}

// findProfile returns a copy of the profile, not a pointer into the document.
//
// That is deliberate. Handing back &c.Profiles[i] invites `p.Rules = next`, which writes
// through the slice a concurrent reader may still be holding -- the hazard internal/config's
// copy-on-write helpers exist to close, and closing it inside the helper while reopening it
// here would be worse than not having it. The caller mutates the copy and hands the whole
// thing to UpsertProfile, which is the exported way to replace an element.
func findProfile(c *config.Config, id string) (config.Profile, bool) {
	for _, p := range c.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return config.Profile{}, false
}

// freshRuleID turns a name into an id that is unique inside the profile.
//
// The slug is only a starting point: two rules called "School night" must not collide, and a
// duplicate id is a document Validate refuses. So the first collision adds a number rather
// than silently reusing the id.
func freshRuleID(rules []config.Rule, name string) string {
	base := slug(name)
	if base == "" {
		base = "rule"
	}
	candidate := base
	for n := 2; ; n++ {
		taken := false
		for _, r := range rules {
			if r.ID == candidate {
				taken = true
				break
			}
		}
		if !taken {
			return candidate
		}
		candidate = base + "-" + strconv.Itoa(n)
	}
}

// slug keeps the characters a config file is pleasant to read and drops the rest. Account
// and rule names are whatever a parent types, including accents and punctuation.
func slug(name string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_':
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func sessionName(s *Server, r *http.Request) string {
	if sess, ok := s.sessionFrom(r); ok {
		return sess.Username
	}
	return "unknown"
}
