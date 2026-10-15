package web

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"

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
	// Data is the view model. The templates reach it as ".", because the alternative
	// -- writing "{{.Data.Banner}}" in twelve places -- makes the markup unreadable
	// and hides what the page actually shows.
	Data any
}

// pageTemplates holds the parsed pages, keyed by name.
//
// Parsing happens once per process rather than per request. The templates are
// embedded, so they cannot change while the daemon runs, and re-parsing on every
// request would mean a syntax error in a .tmpl file is discovered by whichever
// request happens to arrive rather than at startup -- and a daemon that answers 500
// with nothing in the log is the failure mode this project keeps having to design
// against.
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
	if err := t.ExecuteTemplate(&buf, "page", pageData{
		CSS:  s.assetURL("app.css"),
		JS:   s.assetURL("app.js"),
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

func (s *Server) readAccounts() ([]users.Account, error) {
	if s.Accounts == nil {
		return nil, errors.New("no account source is wired")
	}
	return s.Accounts()
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
