package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// The seams are the only place this package touches the machine, and the coverage for
// them asserted only that a page failed. Four mutants survived on that: removing any
// one guard still produced a non-200, because the next guard or a nil-func panic
// produced one anyway. Each guard below is pinned by the sentence it writes, not by the
// status code.

// pageWithSeams builds a page-serving server and hands back the log sink with it.
// pageServer throws the sink away, which is precisely why the log-only assertions had
// no home and the session-failure line went unasserted.
func pageWithSeams(t *testing.T) (*Server, *logSink) {
	t.Helper()
	s, logged, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	m, err := AssetMap(Assets())
	if err != nil {
		t.Fatalf("the embedded assets are not readable: %v", err)
	}
	s.assets = m
	s.Now = func() time.Time { return insideWindow }
	s.Config = func() (*config.Config, error) { return pageConfig(), nil }
	s.Lockables = func() ([]platform.Session, error) { return nil, nil }
	// A wired source that found nobody -- not a nil return, which readAccounts now
	// refuses as "answered nothing at all" rather than "nobody is here".
	s.Accounts = func() (*users.Listing, error) {
		return &users.Listing{UIDMin: users.DefaultUIDMin, UIDMax: users.DefaultUIDMax}, nil
	}
	return s, logged
}

func TestEachUnwiredSeamNamesItself(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		drop func(*Server)
		want string
	}{
		{
			name: "no configuration source",
			path: "/",
			drop: func(s *Server) { s.Config = nil },
			want: "no configuration source is wired",
		},
		{
			name: "no account listing",
			path: "/users",
			drop: func(s *Server) { s.Accounts = nil },
			want: "no account source is wired",
		},
		{
			// The session column is the tolerated one, so this row does two jobs: the
			// guard has to fire, and the page has to survive it. Without the guard a nil
			// Lockables is a call through a nil func -- a panic, a recovered 500, and a
			// dashboard that should have rendered anyway.
			name: "no session source",
			path: "/",
			drop: func(s *Server) { s.Lockables = nil },
			want: "no session source is wired",
		},
		{
			// The audit page is the one that would render this as an empty table, which
			// is a claim about what happened rather than about the wiring.
			name: "no audit trail",
			path: "/audit",
			drop: func(s *Server) { s.Trail = nil },
			want: "no audit trail is wired",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, logged := pageWithSeams(t)
			tc.drop(s)
			authGet(t, s, tc.path)
			if got := strings.Join(logged.Lines(), "\n"); !strings.Contains(got, tc.want) {
				t.Errorf("the seam was not named in the log, want %q:\n%s", tc.want, got)
			}
		})
	}
}

// authGetAfterBreaking signs in while the server still works, breaks it, then asks for
// the page.
//
// The order is the whole point. The login path reads its credential through the same
// Config seam the pages do, so a test that broke the seam first would be refused at
// sign-in and then "pass" its page assertion for a reason it was never about -- the
// masking shape again, arriving through a shared seam rather than a stronger guard.
func authGetAfterBreaking(t *testing.T, s *Server, breakIt func(*Server), path string) *httptest.ResponseRecorder {
	t.Helper()
	resp, _ := login(t, s, "parent", "hunter2")
	c := cookieOf(t, resp)
	breakIt(s)
	req := httptest.NewRequest("GET", path, nil)
	req.AddCookie(c)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// A source that answers "no error" and hands back nothing is a different fault from an
// unwired source, and it is the worse one: the page would render an empty machine.
func TestAConfigSourceThatReturnsNothingIsRefused(t *testing.T) {
	s, logged := pageWithSeams(t)
	rec := authGetAfterBreaking(t, s, func(s *Server) {
		s.Config = func() (*config.Config, error) { return nil, nil }
	}, "/")
	if rec.Code == http.StatusOK {
		t.Fatal("a nil configuration rendered a dashboard")
	}
	if got := strings.Join(logged.Lines(), "\n"); !strings.Contains(got, "returned no configuration") {
		t.Errorf("the empty result was reported as some other fault:\n%s", got)
	}
}

// The page is the machine's current state, served to a phone that may be on the same
// wifi as the child it describes. A cached dashboard can tell a parent their kid is
// locked out after an override expired, or the reverse.
func TestAPageIsNeverCacheable(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	rec := authGet(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("the dashboard did not render: %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q on a rendered page, want no-store", got)
	}
}

// The same header on the success path, asserted at the writer rather than through the
// middleware chain.
//
// The middleware already puts no-store on everything that is not a hashed asset, so a
// request through the router shows the header whether or not render sets it -- the
// stronger guard hides this one completely. That is worth keeping in mind rather than
// "fixing": render is also called from paths that do not run the middleware, and a
// page writer that depends on someone else having set the header is one refactor away
// from serving a cacheable dashboard.
func TestThePageWriterSetsNoStoreItself(t *testing.T) {
	s, _ := pageWithSeams(t)
	rec := httptest.NewRecorder()
	s.render(rec, httptest.NewRequest("GET", "/users", nil), "users", usersPage{})
	if rec.Code != http.StatusOK {
		t.Fatalf("the users page did not render: %d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q written straight from render, want no-store", got)
	}
}

// The refusal path carries the same header: "the configuration could not be read" is
// exactly the answer a cache must not hold on to and replay after the file is fixed.
func TestAPageErrorIsNeverCacheableEither(t *testing.T) {
	s, _ := pageWithSeams(t)
	rec := authGetAfterBreaking(t, s, func(s *Server) {
		s.Config = func() (*config.Config, error) { return nil, errors.New("config file is unreadable") }
	}, "/")
	if rec.Code == http.StatusOK {
		t.Fatal("the page rendered despite the configuration error")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q on a refused page, want no-store", got)
	}
}

// The session failure is the one the page survives, which makes it the one most likely
// to be swallowed. The existing test asserts the page still renders and still says
// "no desktop session"; this is the other half -- that the daemon said so out loud,
// with the reason, because the operator is the only one who can fix a dead bus.
func TestTheSessionFailureIsSaidOutLoud(t *testing.T) {
	s, logged := pageWithSeams(t)
	s.Lockables = func() ([]platform.Session, error) {
		return nil, errors.New("dbus: transport not connected")
	}
	rec := authGet(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("the whole page went down for the session column: %d", rec.Code)
	}
	got := strings.Join(logged.Lines(), "\n")
	if !strings.Contains(got, "session list unavailable") {
		t.Errorf("the session failure was swallowed:\n%s", got)
	}
	if !strings.Contains(got, "dbus: transport not connected") {
		t.Errorf("the failure was reported without its reason:\n%s", got)
	}
}

// A template that cannot execute its data is a programming error. The request must not
// half-render, and the log must name the page.
func TestARenderFailureIsReportedNotHalfWritten(t *testing.T) {
	s, logged := pageWithSeams(t)
	rec := httptest.NewRecorder()
	// users.tmpl reaches for .Data.Rows; anything without that field fails mid-render.
	s.render(rec, httptest.NewRequest("GET", "/users", nil), "users", struct{}{})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for a page that could not render", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<table") {
		t.Error("a failed render wrote page markup to the response")
	}
	got := strings.Join(logged.Lines(), "\n")
	if !strings.Contains(got, "could not render") {
		t.Errorf("the render failure was silent:\n%s", got)
	}
	if !strings.Contains(got, "users") {
		t.Errorf("the render failure did not say which page:\n%s", got)
	}
}
