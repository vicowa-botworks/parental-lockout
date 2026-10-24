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

	ambiguous := config.AmbiguousProfileNames(cfg.Profiles)
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
		// A pair the page cannot tell apart is said here rather than refused at load, because
		// the config still enforces and the operator's way out of it is a write. Refusing to
		// load, or refusing every write, would turn a labelling problem into a lockout nobody
		// can lift. The write paths refuse to create the pair; this says one is already here.
		if ambiguous[p.ID] {
			if row.Complaint != "" {
				row.Complaint += "; "
			}
			row.Complaint += "another profile is listed under this name too, so these two " +
				"cannot be told apart on this page"
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
// that name. It is the config package's error rather than a web-local one: `plock profile add`
// and `plock profile edit --name` refuse the same thing, and two sentences for one refusal is
// how an operator concludes that only one of the paths is real. It stays a sentinel rather than
// a fmt.Errorf so the handler answers 422 with this sentence instead of one built from the
// colliding name -- see the comment on handleProfileRename.
var errProfileNameTaken = config.ErrProfileNameTaken

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
// formTimezone reads and judges the timezone field, refusing the request itself when the
// value will not do.
//
// It is a function rather than three inline checks because two handlers now take a timezone
// on the way in -- the rename page's zone field and the create form -- and the three refusals
// below are the whole of what this machine accepts. A second copy of that list, one per
// handler, is a list that drifts a field at a time, which is the failure the file's own header
// calls out about Validate.
//
// It writes the refusal rather than returning a sentence so that a caller cannot forget to:
// the only thing a caller can do with a false is return.
func (s *Server) formTimezone(w http.ResponseWriter, r *http.Request) (string, bool) {
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
		return "", false
	}
	if _, err := time.LoadLocation(zone); err != nil {
		// The typed value is deliberately not echoed back. It came from the request, and
		// reflecting request text into a response body is a habit this package does not
		// have even when the response is plain text (see handleDeleteProfile).
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			"that is not a timezone this machine can load; use a zone name such as "+
				"Europe/Paris, from the IANA zone list")
		return "", false
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
		return "", false
	}
	return zone, true
}

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

	zone, ok := s.formTimezone(w, r)
	if !ok {
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
// This is not a web-only guarantee, and it used to be wrong to say it was. `plock profile add
// --copy-from` does carry the source's name onto the copy whenever --name is omitted, which is
// the one way to build the pair from a shell -- but that path now calls the same predicate
// (cmd/plock/manage.go, in profileAdd and in profile edit --name), so the CLI refuses it too.
// The sentence here used to claim the opposite and file the hole as open; #127 closed it.
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
		// The rule is config.ProfileNameTaken, shared with `plock profile add` and
		// `plock profile edit --name`: the config is writable from all three, and a check that
		// only the form makes is a check the CLI can walk past with one command.
		if config.ProfileNameTaken(c.Profiles, name, p.ID) {
			return errProfileNameTaken
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
			s.pageRefuse(w, r, http.StatusUnprocessableEntity, errProfileNameTaken.Error())
		default:
			s.mutateRefused(w, r, "the profile could not be renamed", err)
		}
		return
	}

	s.audit("profile.renamed", fmt.Sprintf("profile %s %q -> %q by %s",
		id, previous, name, sessionName(s, r)))
	http.Redirect(w, r, "/profiles", http.StatusSeeOther)
}

// handleCreateProfile adds a profile with no windows in it.
//
// The id is derived, never typed. config.UpsertProfile replaces whatever already carries the id
// it is handed, and it is the seam every profile write on this page goes through, so a create
// form that took an id from the operator would be an overwrite path: typing an id already in use
// would replace that profile's windows and silently re-decide every account pointing at it,
// while the response looked like a successful create. freshProfileID makes that unreachable by
// typing, exactly as freshRuleID makes a duplicate rule id unreachable on the form above.
//
// The name is still refused when it is taken. Validate checks ids and not names, and it is the
// page that makes a duplicate cost something: two profiles under one name render two identical
// headings, two identical table captions and two identical delete confirmations for two
// different documents. The predicate is config.ProfileNameTaken, the same one `plock profile
// add` and `plock profile edit --name` call, because a rule only the form enforces is a rule the
// CLI walks past with one command.
func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.pageRefuse(w, r, http.StatusBadRequest, "the form could not be read")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			"a new profile needs a name; it is what this page, the accounts page and the audit "+
				"trail all point at")
		return
	}
	zone, ok := s.formTimezone(w, r)
	if !ok {
		return
	}

	created := ""
	if err := s.mutateConfig(func(c *config.Config) error {
		if config.ProfileNameTaken(c.Profiles, name, "") {
			return errProfileNameTaken
		}
		created = freshProfileID(c.Profiles, name)
		// No rules. A profile with none is a document Validate accepts and the page says out
		// loud locks nobody out, so an empty create is an honest starting state rather than a
		// half-finished write -- and it is the state a parent in a hurry actually wants, so
		// that the windows can be added beside the row instead of being typed into a form
		// that also has to guess the timezone.
		c.UpsertProfile(config.Profile{ID: created, Name: name, Timezone: zone})
		return nil
	}); err != nil {
		if errors.Is(err, errProfileNameTaken) {
			s.pageRefuse(w, r, http.StatusUnprocessableEntity, errProfileNameTaken.Error())
			return
		}
		s.mutateRefused(w, r, "the profile could not be created", err)
		return
	}

	s.audit("profile.created", fmt.Sprintf("profile %s (%s) in %s by %s",
		created, name, zone, sessionName(s, r)))
	http.Redirect(w, r, "/profiles", http.StatusSeeOther)
}

// handleCopyProfile adds a profile that starts out as another profile's windows.
//
// Only the copy's name is asked for. The timezone deliberately does not come from the form: a
// rule is a wall-clock time whose meaning depends on the zone it is read in, so a copy that took
// a fresh zone from a field while carrying the source's rule text would move every window by the
// offset between the two zones and still look identical to the profile it came from. Inheriting
// the source's zone is the one choice that cannot be wrong, and moving it afterwards is what the
// timezone form on the new row is for -- a form that already says what moving it does.
//
// The rules cross as a fresh slice. They are the source's own element contents, and appending
// into that backing array later -- which is what handleAddRule would do if handed the source's
// slice -- would write through a profile somebody else may still be holding. See findProfile.
func (s *Server) handleCopyProfile(w http.ResponseWriter, r *http.Request) {
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
		// The copy cannot keep the source's name: that is the duplicate the rename path
		// refuses, and `plock profile add --copy-from` refuses too. Asking is cheaper than
		// inventing a suffix nobody recognises when the page lists it.
		s.pageRefuse(w, r, http.StatusUnprocessableEntity,
			"the copy needs its own name; two profiles under one name cannot be told apart on "+
				"this page")
		return
	}

	created, windows := "", 0
	if err := s.mutateConfig(func(c *config.Config) error {
		src, ok := findProfile(c, id)
		if !ok {
			return errUnknownProfile
		}
		if config.ProfileNameTaken(c.Profiles, name, "") {
			return errProfileNameTaken
		}
		created = freshProfileID(c.Profiles, name)
		windows = len(src.Rules)
		c.UpsertProfile(config.Profile{
			ID:       created,
			Name:     name,
			Timezone: src.Timezone,
			Rules:    append([]config.Rule(nil), src.Rules...),
		})
		return nil
	}); err != nil {
		switch {
		case errors.Is(err, errUnknownProfile):
			s.pageRefuse(w, r, http.StatusNotFound, "no profile on this machine has that id")
		case errors.Is(err, errProfileNameTaken):
			s.pageRefuse(w, r, http.StatusUnprocessableEntity, errProfileNameTaken.Error())
		default:
			s.mutateRefused(w, r, "the profile could not be copied", err)
		}
		return
	}

	s.audit("profile.copied", fmt.Sprintf("profile %s (%s) copied from %s, %d windows, by %s",
		created, name, id, windows, sessionName(s, r)))
	http.Redirect(w, r, "/profiles", http.StatusSeeOther)
}

// freshProfileID turns a name into an id that no profile on this machine uses.
//
// The same shape as freshRuleID, for the same reason and one more: a duplicate id is a document
// Validate refuses, and here a duplicate would also be an overwrite, because the write that
// stores the new profile is UpsertProfile. So the slug is a starting point rather than a promise
// -- two profiles called "Kids" end up as kids and kids-2.
func freshProfileID(profiles []config.Profile, name string) string {
	base := slug(name)
	if base == "" {
		base = "profile"
	}
	candidate := base
	for n := 2; ; n++ {
		taken := false
		for _, p := range profiles {
			if p.ID == candidate {
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
