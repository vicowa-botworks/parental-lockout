package web

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

//go:embed templates/*.tmpl
var pageFS embed.FS

// pageData wraps a view model with the two things the shell needs and no view model
// has any reason to know about.
type pageData struct {
	CSS string
	JS  string
	// CSRF is the session's token, emitted twice: as a hidden field on the shell's
	// logout form and as the `meta name="csrf-token"` that static/app.js reads.
	//
	// It belongs on the shell rather than on each view because every page carries the
	// same sign-out form, and a control that cannot post is not a control. Without it
	// the one interactive element in the shell 403s against csrfMiddleware on every
	// page -- and every logout test written by hand-building a `csrf=<token>` body
	// stays green while it does.
	CSRF string
	// Path is the request's own path, and it is the only thing in the shell that says which
	// page the shell is drawing a frame around (#204).
	//
	// It is here rather than derived from the page's template name because the two are not the
	// same question. `pagename` names a template, so it answers "which file rendered this" --
	// and the two POST result pages are rendered by a template named for the action that
	// produced them, at a URL that is a child of an account row. The navigation does not care
	// which file ran; it cares which section of the site the reader is standing in, and only
	// the URL knows that.
	//
	// It is filled by `render` for the same reason CSRF is: a page cannot be allowed to forget
	// it, and a shell that renders it as the empty string marks nothing (see IsCurrent).
	Path string
	// Data is the view model. The templates reach it as ".", because the alternative
	// -- writing "{{.Data.Banner}}" in twelve places -- makes the markup unreadable
	// and hides what the page actually shows.
	Data any
}

// IsCurrent is the shell's navigation asking, once per entry, whether that entry is where the
// reader already is (#204).
//
// One method rather than a conditional written at each of the five links, because the answer is
// one rule and five copies of a rule is the shape #202 was filed about: this file's own history
// is a stylesheet with one selector naming one input type, where the shared rule should have
// been. Written here it is also testable without a browser -- internal/web/nav_test.go drives it
// through the real handlers -- and `aria-current` and the stylesheet hook can never disagree,
// because there is only one of them.
//
// An empty Path marks nothing. That is the refusal, not an oversight: the shell asking "which
// page is this" about a request with no path has no answer, and marking the site's own root as
// current on the strength of a missing value would be the same class of mistake as a nil listing
// rendering "no accounts" (see readAccounts).
func (d pageData) IsCurrent(href string) bool {
	if d.Path == "" {
		return false
	}
	// A navigation entry names exactly one section -- "/" the root, "/users" the accounts --
	// and no two entries in the shell name the same one. So comparing one segment is enough to
	// make the answer true for one entry and false for the rest, with no list of the nav's
	// hrefs anywhere for a future entry to be forgotten by.
	return firstSegment(href) == firstSegment(d.Path)
}

// firstSegment is the section of the site a path is in: "/users/row/test-lock" and "/users/" and
// "/users" are all "users", and "/" -- and nothing else -- is "".
//
// One split is the whole rule, and it is deliberately a split rather than a prefix test: a prefix
// test makes "/" a prefix of every path, which would mark two entries on any page under a section,
// and it makes "/users" a prefix of "/usersfoo", which would mark an account page on a path nobody
// registered. It also handles the trailing slash without being told to: "/users/" splits at the slash
// the empty second segment leaves behind, so it answers "users" exactly as "/users/casey/lock-now"
// does, and "/" is empty before the split ever runs. There is therefore no `TrimSuffix` here -- the
// one that was here until #229's review measured it was unreachable, and unpinable in the same
// breath: deleting it changed no output, no test, and not even the `{"/users/", …}` case written to
// cover it. A line no input reaches is not a guard, and a comment that credits one with doing work
// is how the next reader keeps it.
//
// A path with no leading slash ("") is "" too, which IsCurrent refuses before it gets here.
func firstSegment(p string) string {
	p = strings.TrimPrefix(p, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return p
}

// pageTemplates holds the parsed pages, keyed by name.
//
// Parsing happens once per process rather than per request. The templates are embedded,
// so they cannot change while the daemon runs.
//
// sync.OnceValue is LAZY, so this declaration alone does not parse anything at startup;
// New calls parsePagesAtStartup to get that. Without it a syntax error in a .tmpl is
// found by whichever request happens to arrive, and worse than late: a panic inside the
// OnceValue still marks it done, so the first request panics and every request after it
// takes the "no template named X is embedded" branch -- a message pointing at a missing
// file rather than the parse failure that actually happened, forever, with nothing in the
// log to say so.
var pageTemplates = sync.OnceValue(func() map[string]*template.Template {
	shell, err := fs_read(pageFS, "templates/page.tmpl")
	if err != nil {
		panic("web: the page shell is not readable: " + err.Error())
	}
	pages := map[string]*template.Template{}
	names, err := fs_glob(pageFS, "templates/*.tmpl")
	if err != nil {
		panic("web: the page templates are not readable: " + err.Error())
	}
	for _, name := range names {
		if strings.HasSuffix(name, "page.tmpl") {
			continue
		}
		body, err := fs_read(pageFS, name)
		if err != nil {
			panic("web: " + name + " is not readable: " + err.Error())
		}
		// The shell is parsed into a fresh set per page rather than shared, because
		// each page defines the blocks the shell invokes. One set holding every
		// page's definitions would let the last one parsed win, and every URL would
		// render the same body.
		t, err := template.New("page").Parse(shell + "\n" + body)
		if err != nil {
			panic("web: " + name + " does not parse: " + err.Error())
		}
		base := strings.TrimSuffix(strings.TrimPrefix(name, "templates/"), ".tmpl")
		pages[base] = t
	}
	if len(pages) == 0 {
		panic("web: no page templates are embedded")
	}
	return pages
})

// fs_read and fs_glob are thin wrappers so the panics above name the file they
// failed on rather than returning an error nobody can act on.
func fs_read(f embed.FS, name string) (string, error) {
	b, err := f.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func fs_glob(f embed.FS, pattern string) ([]string, error) {
	return fs.Glob(f, pattern)
}

// render writes a page.
//
// The body is buffered and only written once it is known to be complete. A template
// that fails halfway -- a nil field, a bad method call -- would otherwise leave a
// half-written document on the wire with a 200 already sent, and a browser renders
// half a table as a whole table. Buffering costs one allocation per request on a
// page a human loads by hand.
// parsePagesAtStartup turns the templates' parse-time panic into a startup error.
//
// The templates are embedded, so a parse failure is a build-time bug rather than
// something an operator can cause -- but the alternative is a daemon that comes up and
// answers 500 forever with a message about a missing file, which is the failure mode this
// project keeps having to design against. Refusing to start is the readable version of
// the same fact.
func parsePagesAtStartup() (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("web: the page templates are unusable: %v", p)
		}
	}()
	pageTemplates()
	return nil
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, view any) {
	t, ok := pageTemplates()[name]
	if !ok {
		// Not reachable from a route: the routes name pages that are compiled in.
		// Reaching it means somebody added a handler without adding its template, and
		// the loud answer is the useful one.
		s.logf("web: %s no template named %q is embedded", requestID(r), name)
		http.Error(w, "page unavailable", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	// The session is always present here: every page route sits behind the auth gate,
	// and the CSRF layer has already matched this token against the session. Asking
	// again rather than threading it through keeps the shell honest -- a page rendered
	// with no session gets an empty token, and the form it renders then fails the same
	// way an attacker's forged post does, which is the safe direction.
	csrf := ""
	if sess, ok := s.sessionFrom(r); ok {
		csrf = sess.CSRFToken
	}
	// A page that posts something back needs the token in its form body, because
	// csrfMiddleware reads the `csrf` field and a form cannot set a header. The shell
	// passes only .Data into a page body, so the token is not otherwise reachable
	// there -- and a body that silently omits it renders a form that can never be
	// accepted, which is the defect #59's review found on the logout button.
	//
	// Filling it here rather than in each handler means a page cannot forget it. The
	// interface is asserted, not required: most pages have no forms and should not
	// carry a field they never use.
	if cc, ok := view.(csrfCarrier); ok {
		cc.setCSRF(csrf)
	}
	if err := t.ExecuteTemplate(&buf, "page", pageData{
		CSS:  s.assetURL("app.css"),
		JS:   s.assetURL("app.js"),
		CSRF: csrf,
		// The path the request came in on, not the name of the template about to render it:
		// the navigation marks the section the reader is in, and a POST result page is a child
		// of the URL it was posted to. Raw path, no query -- a query is not part of a location,
		// and a filter on /audit must not stop the Audit link from marking itself.
		Path: r.URL.Path,
		Data: view,
	}); err != nil {
		s.logf("web: %s %s could not render: %v", requestID(r), name, err)
		http.Error(w, "page unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The page is the machine's current state and it is served to a phone that may be
	// on the same wifi as the child it describes. Caching it means a parent can be
	// shown a stale answer about whether their kid is locked out, and no-store also
	// keeps it out of a shared proxy's cache.
	w.Header().Set("Cache-Control", "no-store")
	_, _ = buf.WriteTo(w)
}

// pageError is the failure page.
//
// It says which read failed rather than printing the exception, because the operator
// reading it is the person who can do something about a missing config file, and the
// exception text may carry a path they would only copy into a bug report. The detail
// goes to the log, where it is useful with the request id next to it.
func (s *Server) pageError(w http.ResponseWriter, r *http.Request, what string, err error) {
	s.logf("web: %s %s: %v", requestID(r), what, err)
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, what, http.StatusServiceUnavailable)
}

// readConfig, readSessions and readAccounts turn a missing seam into an error rather
// than a zero value.
//
// The three are separate on purpose. A page that rendered "no accounts" when the
// listing seam was nil would be stating a fact about the machine that the daemon has
// not established, and "nobody is managed" is the most reassuring wrong answer this
// interface can produce.

func (s *Server) readConfig() (*config.Config, error) {
	if s.Config == nil {
		return nil, errors.New("no configuration source is wired")
	}
	cfg, err := s.Config()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, errors.New("the configuration source returned no configuration")
	}
	return cfg, nil
}

func (s *Server) readSessions() ([]platform.Session, error) {
	if s.Lockables == nil {
		return nil, errors.New("no session source is wired")
	}
	return s.Lockables()
}

func (s *Server) readAccounts() (*users.Listing, error) {
	if s.Accounts == nil {
		return nil, errors.New("no account source is wired")
	}
	l, err := s.Accounts()
	if err != nil {
		return nil, err
	}
	if l == nil {
		// A nil listing with a nil error is not "nobody was found"; it is a source that
		// answered nothing at all, and rendering it would say "no accounts" about a
		// read that produced no answer.
		return nil, errors.New("the account source returned no listing")
	}
	return l, nil
}

// assetURL is the hashed URL for one embedded file, or "" when it is not in the
// bundle.
//
// The map is keyed by URL, so this is a scan -- over a handful of entries, once per
// request, on a page a human loads by hand. Building a second index would be the
// right move at a hundred assets and is noise at two.
//
// An empty string makes the shell omit the tag rather than emit a broken link: a
// stylesheet that 404s looks like a broken daemon, which is the exact thing embedding
// the assets was done to prevent.
func (s *Server) assetURL(name string) string {
	for url, file := range s.assets {
		if file == name {
			return url
		}
	}
	return ""
}

// pageNames is the embedded page list, sorted. It exists so a test can assert every
// route has a template instead of finding out by clicking.
func pageNames() []string {
	out := []string{}
	for n := range pageTemplates() {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Trail reads the audit log.
//
// It is an interface rather than an *audit.Reader because the reader's whole job is
// opening files, and a page whose data source is a path on disk can only be tested by
// arranging that disk -- which is how a page about what the daemon did ends up asserting
// facts about the CI runner's filesystem instead of about the page.
//
// Only Tail is here, because that is all this page does. Carrying Generations and
// NewestUser on the assumption that a later page wants them would oblige every fake in
// the package to answer questions nothing asks.
type Trail interface {
	Tail(n int) (audit.Tail, error)
}

// readTrail asks the seam for the newest n records.
//
// A nil seam is an error rather than an empty Tail. The difference is the whole point: an
// empty Tail renders "the trail is empty", which is a claim about what happened, and a
// daemon started without a trail wired has made no claim about anything. The same rule
// already governs readAccounts, where a nil listing with a nil error is refused for
// exactly this reason.
func (s *Server) readTrail(n int) (audit.Tail, error) {
	if s.Trail == nil {
		return audit.Tail{}, errors.New("no audit trail is wired")
	}
	return s.Trail.Tail(n)
}
