package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The settings page: the enforcement master switch and the tick interval, which are the two
// values a parent reaches for when the daemon is misbehaving and which, until now, could only be
// changed by editing /etc/plock/config.json as root.
//
// The master switch is the reason the page exists. The dashboard already *reports* it --
// dashboard.Build sets EnforcementOn and the banner says "nothing is being locked out" -- so
// every operator who has read that sentence has also had no way to act on the one control it
// describes. A screen that knows about a switch and cannot touch it teaches the parent that the
// way out of a misbehaving daemon is a terminal and a JSON editor, which is the moment a tool
// like this gets routed around.
//
// Scope is deliberately those two fields, not the whole of config.Settings. `listen` and
// `lockBackends` change what the daemon binds and what it shells out to, and neither has a story
// yet for "you just made the UI unreachable"; `tls` and the certificate paths are the third
// slice of #99 for the reason that issue gives -- a tls mode naming a certificate that does not
// exist must be refused before it is saved, because the next daemon start would fail on a config
// this UI accepted.

// settingsFieldPoll is the form field name for the tick interval. It matches the JSON key
// (`settings.pollSeconds`) on purpose: the message naming the field is then the same string the
// operator would grep for in the config file, and the two surfaces cannot end up naming one
// setting two things.
const settingsFieldPoll = "pollSeconds"

// errNothingChanged is how the mutation tells the handler that the submitted document is the
// document already on disk.
//
// It is reported from inside the mutation rather than computed by the handler because that is the
// only moment the answer is knowable: the seam loads the file again under the write lock, so a
// concurrent `plock` edit between the handler's read and the seam's load changes what "changed"
// means, and a diff taken from the earlier read would report a change that is not happening. The
// sentinel also keeps the seam from renaming the config file just to say it did.
var errNothingChanged = errors.New("the submitted settings are already saved")

// settingsPage is what settings.tmpl renders.
type settingsPage struct {
	// Enforcement and PollSeconds are the values the page speaks as current.
	Enforcement bool
	PollSeconds int

	// PollField is what goes in the interval box. On a refused submit it is the text that
	// was typed, not the stored number: an operator who typed 2s and is told "pollSeconds
	// is not a number" needs to see their own 2s to know what the daemon saw, and a box that
	// silently reverts to 30 makes the refusal look like it was about something else.
	PollField string

	// MinPoll and MaxPoll are config's own constants, so the page states the real bounds
	// rather than a copy that goes stale when validate.go moves them.
	MinPoll int
	MaxPoll int

	// Problem names what the operator has to fix; Message names what just happened. They are
	// mutually exclusive on every path that sets one, and a page rendering both would be
	// refusing and accepting the same submit.
	Problem string
	Message string

	// CSRF is filled by render through csrfCarrier, never by the handler.
	CSRF string
}

func (p *settingsPage) setCSRF(token string) { p.CSRF = token }

// settingsView builds the page from a document the caller already read.
//
// It takes the config rather than reading it so a test can assert on the rendering -- including
// the bounds, which are the page's only claim about what the daemon accepts -- without a request.
func settingsView(cfg *config.Config) settingsPage {
	return settingsPage{
		Enforcement: cfg.Settings.Enforcement,
		PollSeconds: cfg.Settings.PollSeconds,
		PollField:   strconv.Itoa(cfg.Settings.PollSeconds),
		MinPoll:     config.MinPollSeconds,
		MaxPoll:     config.MaxPollSeconds,
	}
}

// handleSettings is GET /settings.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}
	page := settingsView(cfg)
	// The pointer, not the value: render fills the token through setCSRF, and a copy would fill a
	// temporary the template never sees -- every submit would then be refused as a CSRF failure,
	// which is #59's defect arriving through a new page.
	s.render(w, r, "settings", &page)
}

// handleSettingsSave is POST /settings.
//
// Every refusal path re-renders the form rather than answering in plain text, and that is a
// deliberate departure from the override and profile routes, which use pageRefuse. Those refuse a
// request that names the wrong *thing* -- no such account, no such profile -- where the page
// cannot help and a sentence is the whole answer. This page refuses a value inside a form the
// operator is holding: the interval box is what has to change, so the box has to still be there,
// holding what they typed, next to a sentence naming it. A plain-text refusal would send them
// back through the navigation to retype a number.
//
// The status is still 422, because the line this repo draws between 4xx and 5xx is about who has
// to act, and a refusal the operator caused is not something the server can retry.
func (s *Server) handleSettingsSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.pageRefuse(w, r, http.StatusBadRequest, "the form could not be read")
		return
	}
	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}

	// The master switch arrives as a radio pair, not a checkbox, and that is the one place in
	// this UI where the two differ in safety rather than in looks. Every other checkbox here is
	// written so that absence is the safe direction: a rule form that lost its allDay box
	// enforces a timed window instead of an all-day one, and an assignment row that lost its box
	// stops managing an account rather than silently starting to. This switch is the opposite
	// shape -- an absent enforcement field means "stop locking children out" -- so absence cannot
	// be a value at all. A truncated body, a browser that dropped the control, or a curl without
	// the field has to be a refusal, because the silent reading is the fail-open one and this
	// project's rule is that ambiguity resolves toward enforcing (#873). An unrecognised value is
	// refused for the same reason: guessing "off" would be guessing.
	wantEnforcement, refusal := settingsSwitch(r.FormValue("enforcement"))
	if refusal != "" {
		s.renderSettingsRefusal(w, r, cfg, refusal, r.FormValue(settingsFieldPoll))
		return
	}

	// The interval is checked here for the two shapes only the form knows about -- empty, and
	// not a number -- and against config's own constants for the range. The constants are the
	// point: re-typing 5 and 86400 into this file would put a second, weaker copy of
	// validate.go's list in the tree, which is the drift profiles.go warns about by name.
	// Validate still runs inside mutateConfig on the document about to be saved, so what follows
	// is a better sentence about the field, not a replacement for the check.
	rawPoll := strings.TrimSpace(r.FormValue(settingsFieldPoll))
	if rawPoll == "" {
		s.renderSettingsRefusal(w, r, cfg,
			"settings.pollSeconds was empty. The tick interval has to be a number of seconds between "+
				strconv.Itoa(config.MinPollSeconds)+" and "+strconv.Itoa(config.MaxPollSeconds)+
				"; nothing was changed.", rawPoll)
		return
	}
	wantPoll, err := strconv.Atoi(rawPoll)
	if err != nil {
		s.renderSettingsRefusal(w, r, cfg,
			"settings.pollSeconds must be a whole number of seconds, not "+strconv.Quote(rawPoll)+
				"; nothing was changed.", rawPoll)
		return
	}
	if wantPoll < config.MinPollSeconds || wantPoll > config.MaxPollSeconds {
		s.renderSettingsRefusal(w, r, cfg,
			fmt.Sprintf("settings.pollSeconds %d is outside the accepted range of %d to %d seconds; "+
				"nothing was changed.", wantPoll, config.MinPollSeconds, config.MaxPollSeconds),
			rawPoll)
		return
	}

	// changes and flipped are filled inside the mutation, from the document the seam actually
	// loaded. flipped is the switch's new position only when the switch actually moved, which is
	// what lets the confirmation below be chosen by a bool rather than by pattern-matching the
	// sentence it is printed next to.
	var changes []string
	var flipped *bool
	if err := s.mutateConfig(func(c *config.Config) error {
		if c.Settings.Enforcement != wantEnforcement {
			changes = append(changes, "enforcement "+switchWord(c.Settings.Enforcement)+
				" to "+switchWord(wantEnforcement))
			c.Settings.Enforcement = wantEnforcement
			flipped = &wantEnforcement
		}
		if c.Settings.PollSeconds != wantPoll {
			changes = append(changes, "pollSeconds "+strconv.Itoa(c.Settings.PollSeconds)+
				" to "+strconv.Itoa(wantPoll))
			c.Settings.PollSeconds = wantPoll
		}
		if len(changes) == 0 {
			return errNothingChanged
		}
		return nil
	}); err != nil {
		if errors.Is(err, errNothingChanged) {
			// Not a refusal and not a save. "Nothing changed" is the honest answer and it is a
			// useful one: an operator who pressed Save and reads this knows the daemon is already
			// in the state they wanted, which is the thing they were checking.
			page := settingsView(cfg)
			page.PollField = rawPoll
			page.Message = "Nothing changed, so nothing was written."
			s.render(w, r, "settings", &page)
			return
		}
		s.mutateRefused(w, r, "the settings could not be saved", err)
		return
	}

	// One line per accepted submit, naming each field and both ends of each change. The "who" is
	// here for the same reason it is on every other write this server makes: the trail outlives
	// the conversation in which the switch was flipped, and "who turned enforcement off at 2am"
	// is the question it gets read to answer.
	//
	// This is a different fact from the engine's own enforcement.disabled / enforcement.enabled
	// record, which says the loop noticed the switch on a given tick. Two lines for one flip is
	// correct and worth having: the operator's act and the loop's reaction are separate events a
	// few seconds apart, and a trail holding only the second one cannot tell an operator whether
	// their click was ever saved at all.
	s.audit("settings.changed", fmt.Sprintf("%s by %s", strings.Join(changes, "; "),
		sessionName(s, r)))

	// The page after a save speaks the values this submit wrote, which is what the seam just
	// saved and Validate just accepted. Re-reading the file instead would be more independent,
	// but a re-read that failed would leave nothing to render for an action that succeeded -- and
	// answering 503 there would tell the operator the save failed when the config on disk says it
	// did not.
	page := settingsPage{
		Enforcement: wantEnforcement,
		PollSeconds: wantPoll,
		PollField:   strconv.Itoa(wantPoll),
		MinPoll:     config.MinPollSeconds,
		MaxPoll:     config.MaxPollSeconds,
	}
	page.Message = settingsConfirmation(changes, flipped)
	// Rendered rather than redirected, which is where this route parts company with the override
	// and profile writes. Those land on a page whose table already shows the new state, so the
	// confirmation is redundant and 303 buys a refreshable URL. Here the page they land on is the
	// page they submitted from, and the confirmation is the acceptance criterion: switching
	// enforcement off has to name what stops happening, and a redirect arrives with no memory of
	// what was just switched. A refresh re-posts, and re-posting this form is idempotent by
	// construction -- the second submit finds nothing changed and writes nothing -- so rendering
	// costs nothing and redirecting costs the sentence the feature exists to print.
	s.render(w, r, "settings", &page)
}

// renderSettingsRefusal re-renders the form with the operator's own text in the interval box and a
// sentence naming what is wrong.
//
// The box is echoed from the request and the rest of the page is rendered from the document. That
// split is deliberate: the box must show what the daemon was handed, and everything else must keep
// showing what the machine is actually running, or a refusal page would describe a state that does
// not exist.
func (s *Server) renderSettingsRefusal(w http.ResponseWriter, r *http.Request, cfg *config.Config, problem, typedPoll string) {
	page := settingsView(cfg)
	page.PollField = typedPoll
	page.Problem = problem
	// 422 through a wrapper, because render always lands on 200 and a refusal that arrived
	// "OK" would be indistinguishable from an accepted save to anything not reading the prose --
	// including the operator's own browser history, and any future automation posting these forms.
	s.renderStatus(w, r, "settings", &page, http.StatusUnprocessableEntity)
}

// settingsSwitch reads the master switch's radio pair.
//
// It returns the refusal sentence rather than an error because the caller's job is to render it,
// and because naming the field in the operator's terms is a presentation fact a parse cannot
// carry.
func settingsSwitch(raw string) (on bool, refusal string) {
	switch raw {
	case "on":
		return true, ""
	case "off":
		return false, ""
	case "":
		return false, "settings.enforcement was not submitted, so the daemon cannot tell whether " +
			"you meant to switch enforcement off. Pick a position explicitly; nothing was changed."
	default:
		return false, "settings.enforcement must be either on or off, not " + strconv.Quote(raw) +
			"; nothing was changed."
	}
}

// switchWord keeps the audit line readable instead of saying "true to false".
func switchWord(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// settingsConfirmation turns the accepted changes into the sentence on the page.
//
// The enforcement clause names what stops happening, in the dashboard banner's own words, because
// the operator who just switched enforcement off is about to look at that banner and the two
// sentences have to agree about what the switch does. Anything vaguer -- "settings saved" -- is the
// wording that lets a fail-open control read like a routine edit.
//
// flipped is a *bool rather than a bool so "the switch did not move" is expressible: on a submit
// that only changed the interval, neither clause belongs on the page, and a false here would mean
// "enforcement was just switched off" and print a warning about a thing that did not happen.
func settingsConfirmation(changes []string, flipped *bool) string {
	if len(changes) == 0 {
		return "Nothing changed, so nothing was written."
	}
	var b strings.Builder
	b.WriteString("Saved: ")
	b.WriteString(strings.Join(changes, "; "))
	b.WriteString(".")
	if flipped == nil {
		return b.String()
	}
	if *flipped {
		b.WriteString(" Enforcement is on: an account inside a window is locked out on the " +
			"daemon's next tick, without a restart.")
		return b.String()
	}
	b.WriteString(" Enforcement is switched off, so nothing is being locked out and nothing will " +
		"be, whatever the schedule says, until it is switched back on. The status page says so too, " +
		"and the daemon stops re-asserting locks from its next tick.")
	return b.String()
}

// statusWriter puts a response code on a rendered page.
//
// render writes headers and body together and always lands on 200, which was right for every page
// it served until a form had to be refused *as itself*. Wrapping is the smallest honest way to say
// "this document is a refusal" without changing the signature every other handler calls, and
// without a second copy of render's buffering, CSRF filling and no-store headers -- a duplicated
// render is one fix away from a page that loses the CSP meta tag on one of its two paths.
type statusWriter struct {
	http.ResponseWriter
	code int
	// headerDone records that a status has already gone out, so the wrapper never overwrites a
	// more specific code than its own.
	headerDone bool
}

// WriteHeader is overridden rather than swallowed on purpose: if render ever sets a status itself
// -- and today its failure paths do, through http.Error's 500 -- that status is the more specific
// fact and it wins. Without this, a template that failed to render on a refusal path would be
// reported as a 422 the operator could fix by editing the form.
func (w *statusWriter) WriteHeader(code int) {
	w.headerDone = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.headerDone {
		w.WriteHeader(w.code)
	}
	return w.ResponseWriter.Write(b)
}

func (s *Server) renderStatus(w http.ResponseWriter, r *http.Request, name string, view any, code int) {
	s.render(&statusWriter{ResponseWriter: w, code: code}, r, name, view)
}
