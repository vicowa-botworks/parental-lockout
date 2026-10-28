package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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

	// Notice is the sentence from a refused write, and empty on a normal load. It is part of
	// the page rather than the body of an error because a refusal that also throws away the
	// form it refused makes the parent retype the whole thing from memory (#138).
	Notice string
	// Create echoes the create form's boxes when that form is the one that was refused.
	Create *formValues
}

// formValues is what a refused write carries back into the boxes it came from (#138).
//
// It is used through a pointer everywhere, because "this form was not the one that failed" and
// "this form was submitted empty" have to stay different: the first leaves the box as the
// document has it, the second puts an empty box back.
//
// The values reach the page as a template-escaped `value` attribute and nowhere else. The refusal
// sentence still quotes none of them. formTimezone's comment said reflecting request text into a
// response body is a habit this package does not have, and #138 keeps that half: what changes is
// that the text goes back where it was typed, through html/template's escaping, instead of being
// thrown away with the form.
type formValues struct {
	Name     string
	Timezone string
	Start    string
	End      string
	Days     []string
	AllDay   bool
}

// HasDay answers the checkbox loop. Nil-safe on purpose: every form on the page asks this
// question on every render, and a refused form is the only one that carries answers.
func (f *formValues) HasDay(day string) bool {
	if f == nil {
		return false
	}
	for _, d := range f.Days {
		if d == day {
			return true
		}
	}
	return false
}

// Checked is the all-day box's attribute, spelled the way the template wants it.
func (f *formValues) Checked() string {
	if f == nil || !f.AllDay {
		return ""
	}
	return "checked"
}

// profileEcho says which form on the page a refused write came from and what was in it.
type profileEcho struct {
	// id is the profile whose row holds the form. Empty means the create form, which is
	// above the list and is the only form not inside a row.
	id     string
	rename string
	zone   string
	copyAs string
	rule   *formValues
	create *formValues
}

// renderProfilesRefusal answers a declined write by re-rendering the page with the sentence above
// the form and the parent's own text still in the boxes that produced it (#138).
//
// The split is the one renderSettingsRefusal makes: the echoed boxes come from the request and
// every row, table and "Shared by" line is rendered from the document. A refusal page built out
// of the submitted values would describe a profile that does not exist, which is worse than the
// wall of text it replaces -- the parent would have to work out from the table which of their
// changes landed.
//
// The status code is the caller's, unchanged. render lands on 200, and a refusal that arrived
// "OK" would be indistinguishable from an accepted save to anything not reading the prose --
// including the operator's own browser history and any future automation posting these forms.
func (s *Server) renderProfilesRefusal(w http.ResponseWriter, r *http.Request, status int,
	why string, echo profileEcho) {
	cfg, err := s.readConfig()
	if err != nil {
		// The page cannot be rebuilt without the document, and the refusal still has to be
		// said. Dropping "that name is too long" into a 503 about the config would turn a
		// correct refusal into one the parent never hears the reason for.
		s.pageRefuse(w, r, status, why)
		return
	}
	page := s.profilesView(cfg)
	page.Notice = why
	page.Create = echo.create
	for i := range page.Profiles {
		if page.Profiles[i].ID != echo.id {
			continue
		}
		page.Profiles[i].EchoRename = echo.rename
		page.Profiles[i].EchoZone = echo.zone
		page.Profiles[i].EchoCopy = echo.copyAs
		page.Profiles[i].EchoRule = echo.rule
	}
	s.renderStatus(w, r, "profiles", &page, status)
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
	// The boxes on this row that a refused write puts back the way the parent left them
	// (#138). Four separate fields rather than one struct, because the row carries four
	// different forms and a single echoed value would leak the rename's text into the
	// add-a-window box beside it. Only the form that was submitted is ever set.
	EchoRename string
	EchoZone   string
	EchoCopy   string
	EchoRule   *formValues

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

	// Everything the form carries, taken before anything judges it: a refusal has to put back
	// what was typed, including the fields that were perfectly fine (#138). The add-a-window
	// form is the worst case in the package -- a refusal for one bad clock used to cost seven
	// checkboxes, two times and a name, all of it re-entered from memory on a 360 px screen.
	ruleForm := &formValues{
		Days:   r.Form["days"],
		AllDay: r.FormValue("allDay") != "",
		Start:  strings.TrimSpace(r.FormValue("start")),
		End:    strings.TrimSpace(r.FormValue("end")),
	}
	refuse := func(status int, why string) {
		s.renderProfilesRefusal(w, r, status, why, profileEcho{id: id, rule: ruleForm})
	}

	name := strings.TrimSpace(r.FormValue("name"))
	ruleForm.Name = name
	if name == "" {
		refuse(http.StatusUnprocessableEntity,
			"a rule needs a name; it is what the audit trail and this page both point at")
		return
	}
	if !s.formNameFits(refuse, name) {
		return
	}

	allDay := ruleForm.AllDay
	days := ruleForm.Days
	if len(days) == 0 {
		// Refused here rather than left to Validate, which would report it as a rule
		// that names no day. The form knows a box was never ticked; that is a better
		// sentence than the document-level one.
		refuse(http.StatusUnprocessableEntity,
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
		s.mutateRefusedAs(w, r, refuse, "the rule could not be saved", err)
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
// formNameFits refuses a name the document ceiling will not hold, and reports the refusal itself.
//
// It is separate from formTimezone rather than a fifth check inside it because the name field has
// no shared reader: four handlers each carry their own blank-name sentence, and each of those
// sentences is specific to its form ("two profiles under one name cannot be told apart on this
// page" is true of the copy form and meaningless in the rule form). What is shared is the ceiling,
// so that is the one thing factored out.
//
// It writes the refusal rather than returning a sentence so a caller cannot forget to, for the
// reason formTimezone gives: the only thing a caller can do with a false is return.
//
// The refusal is here as well as in config.Validate on purpose. Validate's sentence is about a
// document ("profiles[0].name: must be at most 200 characters"), which is the right thing to show
// someone who hand-edited a file and the wrong thing to show someone who just typed into a box on
// this page. The two say the same rule in the two voices that fit, and neither is the gate: the
// ceiling is Validate's, so a path that skips this check is still refused before anything is
// written.
func (s *Server) formNameFits(refuse refuser, name string) bool {
	if utf8.RuneCountInString(name) <= config.MaxNameLen {
		return true
	}
	// The name is not quoted in the sentence: it came from the request, and it is by definition
	// too long to put in a response about being too long (see formTimezone on the same point).
	// #138 changed the other half -- the caller's refuser puts it back in the box, escaped, so
	// the parent can shorten what they typed instead of retyping it from memory.
	refuse(http.StatusUnprocessableEntity,
		fmt.Sprintf("that name is too long; it can be at most %d characters, so it fits the "+
			"page that has to show it and the audit trail that has to quote it", config.MaxNameLen))
	return false
}

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
// the only thing a caller can do with a false is return. The refusal goes through the caller's
// refuser rather than straight to the ResponseWriter, which is what lets the same check answer a
// refusal-page handler and a plain one (#138).
func (s *Server) formTimezone(r *http.Request, refuse refuser) (string, bool) {
	zone := strings.TrimSpace(r.FormValue("timezone"))
	if zone == "" {
		// Refused rather than saved as blank, because blank is not "unset" here:
		// time.LoadLocation("") answers UTC. A cleared field would move every window in
		// the profile to UTC while the page still looked like a profile that had simply
		// not been filled in. Validate says the same thing at document level; this is the
		// form-level sentence, and the days checkbox on the rule form set the precedent.
		refuse(http.StatusUnprocessableEntity,
			"a profile needs a timezone; a blank one is read as UTC, which would move every "+
				"window in it without anyone choosing that")
		return "", false
	}
	if _, err := time.LoadLocation(zone); err != nil {
		// The sentence quotes nothing from the request: reflecting request text into a
		// response body is a habit this package does not have even when the response is plain
		// text (see handleDeleteProfile). #138 kept that and moved the value instead -- it goes
		// back into the box as a template-escaped `value` attribute, which is where the parent
		// can act on it, rather than being thrown away with the form.
		refuse(http.StatusUnprocessableEntity,
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
		refuse(http.StatusUnprocessableEntity,
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

	// The box is echoed from the raw field, not from the judged one: the value that was
	// refused is the value the parent has to correct (#138).
	typed := strings.TrimSpace(r.FormValue("timezone"))
	refuse := func(status int, why string) {
		s.renderProfilesRefusal(w, r, status, why, profileEcho{id: id, zone: typed})
	}
	zone, ok := s.formTimezone(r, refuse)
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
		s.mutateRefusedAs(w, r, refuse, "the timezone could not be saved", err)
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
	refuse := func(status int, why string) {
		s.renderProfilesRefusal(w, r, status, why, profileEcho{id: id, rename: name})
	}
	if name == "" {
		refuse(http.StatusUnprocessableEntity,
			"a profile needs a name; leaving the field blank would not clear it, it would "+
				"just make the page fall back to the id")
		return
	}
	if !s.formNameFits(refuse, name) {
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
			// would read as a system fault rather than a collision the operator can fix. The
			// typed name goes back in the box (#138): the fix is to change it, and the box is
			// where that happens.
			refuse(http.StatusUnprocessableEntity, errProfileNameTaken.Error())
		default:
			s.mutateRefusedAs(w, r, refuse, "the profile could not be renamed", err)
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

	// Both boxes are echoed even when only one of them was refused: the create form is two
	// fields and a refusal about the zone should not cost the name (#138).
	createForm := &formValues{
		Name:     strings.TrimSpace(r.FormValue("name")),
		Timezone: strings.TrimSpace(r.FormValue("timezone")),
	}
	refuse := func(status int, why string) {
		s.renderProfilesRefusal(w, r, status, why, profileEcho{create: createForm})
	}

	name := createForm.Name
	if name == "" {
		refuse(http.StatusUnprocessableEntity,
			"a new profile needs a name; it is what this page, the accounts page and the audit "+
				"trail all point at")
		return
	}
	if !s.formNameFits(refuse, name) {
		return
	}
	zone, ok := s.formTimezone(r, refuse)
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
			refuse(http.StatusUnprocessableEntity, errProfileNameTaken.Error())
			return
		}
		s.mutateRefusedAs(w, r, refuse, "the profile could not be created", err)
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
	refuse := func(status int, why string) {
		s.renderProfilesRefusal(w, r, status, why, profileEcho{id: id, copyAs: name})
	}
	if name == "" {
		// The copy cannot keep the source's name: that is the duplicate the rename path
		// refuses, and `plock profile add --copy-from` refuses too. Asking is cheaper than
		// inventing a suffix nobody recognises when the page lists it.
		refuse(http.StatusUnprocessableEntity,
			"the copy needs its own name; two profiles under one name cannot be told apart on "+
				"this page")
		return
	}
	if !s.formNameFits(refuse, name) {
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
			refuse(http.StatusUnprocessableEntity, errProfileNameTaken.Error())
		default:
			s.mutateRefusedAs(w, r, refuse, "the profile could not be copied", err)
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
