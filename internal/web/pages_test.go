package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"github.com/vicowa-botworks/parental-lockout/internal/users"
)

// insideWindow is inside the school-night rule the fixtures below define, so the
// dashboard has something to report.
var insideWindow = time.Date(2026, 3, 18, 23, 0, 0, 0, time.UTC)

// pageVerifier is one production-legal admin credential, minted once for the package.
//
// Production-legal is not a detail: the authenticator checks the stored cost at the
// trust boundary, so a TestIterations fixture is refused there with a 500 -- correctly.
// A page fixture that cannot log in is not testing the page. The cost is one ~100 ms
// hash for the whole package rather than one per fixture, because every page test logs
// in and none of them should pay for the KDF.
var pageVerifier = func() string {
	v, err := auth.Hasher{Iterations: auth.MinIterations}.Hash("hunter2")
	if err != nil {
		panic("the test verifier could not be minted: " + err.Error())
	}
	return v
}()

func pageConfig() *config.Config {
	c := config.Default()
	// A deployable config has an admin in it. This stopped being optional when the
	// login path started reading its credential through the same Config seam the pages
	// use: a fixture that stubbed Config for page data was silently also saying "this
	// machine has no administrator", and every login then failed for a reason the test
	// had nothing to do with.
	c.Admin = config.Admin{Username: "parent", Password: pageVerifier}
	c.Profiles = []config.Profile{{
		ID: "school", Name: "School nights", Timezone: "UTC",
		Rules: []config.Rule{{ID: "r1", Name: "weeknights",
			Days:  []string{"sun", "mon", "tue", "wed", "thu"},
			Start: "21:00", End: "06:30", Enabled: true}},
	}}
	c.Users = []config.User{{Username: "casey", ProfileID: "school", Enabled: true}}
	return c
}

// authGet performs a real login and returns the body of an authenticated GET, so
// none of these tests hand-assemble state the server never issues.
func authGet(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	resp, _ := login(t, s, "parent", "hunter2")
	req := httptest.NewRequest("GET", path, nil)
	// A browser's Accept. Without it the server answers the unauthenticated case as
	// JSON -- which is right for a programmatic caller -- and the test would be
	// asserting a status code no parent's browser ever sees.
	req.Header.Set("Accept", "text/html")
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// pageServer is a server with all three content seams wired to fixed state.
func pageServer(t *testing.T, cfg *config.Config, sessions []platform.Session, accounts []users.Account, cfgErr, acctErr error) *Server {
	t.Helper()
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	// newTestServer assembles a Server by hand rather than through New, so the asset
	// index the shell links the stylesheet from does not exist yet. Building it here
	// is the same call New makes; without it every page renders unstyled and the
	// stylesheet test would be asserting nothing.
	m, err := AssetMap(Assets())
	if err != nil {
		t.Fatalf("the embedded assets are not readable: %v", err)
	}
	s.assets = m
	s.Now = func() time.Time { return insideWindow }
	if cfgErr != nil {
		s.Config = func() (*config.Config, error) { return nil, cfgErr }
	} else {
		s.Config = func() (*config.Config, error) { return cfg, nil }
	}
	s.Lockables = func() ([]platform.Session, error) { return sessions, nil }
	if acctErr != nil {
		s.Accounts = func() (*users.Listing, error) { return nil, acctErr }
	} else {
		s.Accounts = func() (*users.Listing, error) {
			return &users.Listing{Accounts: accounts, UIDMin: users.DefaultUIDMin, UIDMax: users.DefaultUIDMax}, nil
		}
	}
	return s
}

// Nothing outside the login page is readable without a session.
func TestTheContentPagesRequireASession(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	for _, path := range []string{"/", "/users"} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if got := rec.Result().StatusCode; got != http.StatusSeeOther && got != http.StatusFound {
			t.Errorf("GET %s without a session = %d, want a redirect to the login page", path, got)
		}
	}
}

// The dashboard names the account and says what is holding it.
func TestTheDashboardNamesTheAccountAndTheRule(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	rec := authGet(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"casey", "locked out", "weeknights", "06:30"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page never says %q:\n%s", want, body)
		}
	}
}

// A username is attacker-influenced text.
//
// The password file is root-owned, but a machine can carry accounts created by
// somebody else, and an account name is the one field a child with a shell on the box
// can plausibly control. The page must render it as a name.
func TestAnAccountNameIsRenderedAsTextNotMarkup(t *testing.T) {
	c := pageConfig()
	c.Users[0].Username = `<script>alert(1)</script>`
	s := pageServer(t, c, nil, nil, nil, nil)
	body := authGet(t, s, "/").Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("the account name was written into the document as markup")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("the account name is not in the page at all, escaped or otherwise")
	}
}

// The response carries a CSP with no 'unsafe-inline', and the page contains no
// inline handler or style attribute to be blocked by it.
//
// Both halves matter. The header alone means a stray onclick silently stops working
// in a browser while every test still passes; the markup alone means the control is
// absent.
func TestThePageHasNoInlineScriptOrStyleToBeBlockedByTheCSP(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	rec := authGet(t, s, "/")
	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("no Content-Security-Policy header on a page that shows who is locked out")
	}
	if strings.Contains(csp, "unsafe-inline") {
		t.Errorf("the CSP allows inline script, so the markup check below proves nothing: %q", csp)
	}
	body := rec.Body.String()
	for _, bad := range []string{" onclick=", " onload=", " onerror=", " style="} {
		if strings.Contains(strings.ToLower(body), bad) {
			t.Errorf("the page carries %q, which the CSP blocks: the control works in a test and fails in a browser", bad)
		}
	}
}

// A page that cannot read the configuration says so.
//
// An empty table would be a different claim: it reads as "nothing is managed", which
// is the most reassuring wrong answer this interface can produce, and it is exactly
// what a parent sees in the minute when the config file has actually gone missing.
func TestADashboardThatCannotReadTheConfigSaysSo(t *testing.T) {
	s, _ := pageWithSeams(t)
	rec := authGetAfterBreaking(t, s, func(s *Server) {
		s.Config = func() (*config.Config, error) {
			return nil, errors.New("open /etc/plock/config.json: no such file")
		}
	}, "/")
	if rec.Code == http.StatusOK {
		t.Error("the page rendered as though nothing were wrong")
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "configuration") {
		t.Errorf("the refusal does not say what failed: %q", rec.Body.String())
	}
}

// The same for the accounts page, where the missing read is the password file.
func TestTheUsersPageRefusesWhenTheAccountsCannotBeRead(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, errors.New("open /etc/passwd: permission denied"))
	rec := authGet(t, s, "/users")
	if rec.Code == http.StatusOK {
		t.Error("an unreadable account list rendered as an empty one")
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "account") {
		t.Errorf("the refusal does not say what failed: %q", rec.Body.String())
	}
}

// A missing seam is refused the same way a failing read is.
//
// The zero value of `Config` is nil, and a handler that treated nil as "no accounts"
// would let a daemon built without its wiring advertise an empty household forever.
func TestAContentSeamThatWasNeverWiredIsRefused(t *testing.T) {
	s, _, _ := newTestServer(t, &stubAuth{username: "parent", password: "hunter2"})
	rec := authGet(t, s, "/")
	if rec.Code == http.StatusOK {
		t.Error("a server with no configuration source rendered a dashboard anyway")
	}
}

// The session column is the one thing that can be missing without taking the page
// down: which child is held, and until when, does not depend on who is logged in.
//
// What it must not do is say "no desktop session". That sentence is a claim that nobody
// has a session, and a failed read establishes nothing of the kind -- it is the same
// rendering the page gives a machine where nobody is logged in, so a broken session
// backend looks like a quiet house. The page has to distinguish "nobody is logged in"
// from "we could not look", and the two cases are pinned separately below.
func TestTheDashboardStillRendersWhenTheSessionListIsGone(t *testing.T) {
	c := pageConfig()
	s := pageServer(t, c, nil, nil, nil, nil)
	s.Lockables = func() ([]platform.Session, error) { return nil, errors.New("dbus: transport not connected") }
	rec := authGet(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("the whole page went down for the session column: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "casey") {
		t.Error("the accounts vanished with the sessions")
	}
	if !strings.Contains(body, "session list unavailable") {
		t.Errorf("a failed session read is not said as a failure; the body says %q instead, which is "+
			"a claim about who is logged in that the failed read did not establish", "no desktop session")
	}
	if strings.Contains(body, "no desktop session") {
		t.Error("the failed read rendered as an empty session list; a parent cannot tell a broken " +
			"session backend from a machine nobody is logged into")
	}
}

// The other half of the same column: a read that worked and found nobody. This is the
// case "no desktop session" is true for, and it must keep that wording -- otherwise the
// new failure sentence is paid for by making the honest case vague.
func TestAnEmptySessionListStillSaysNobodyIsLoggedIn(t *testing.T) {
	rec := authGet(t, pageServer(t, pageConfig(), nil, nil, nil, nil), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "no desktop session") {
		t.Error("a successful read that found no sessions does not say so")
	}
	if strings.Contains(body, "session list unavailable") {
		t.Error("a read that worked is reported as a failure")
	}
}

// Sessions are shown with the state the backend reported.
func TestTheSessionColumnCarriesTheLockState(t *testing.T) {
	s := pageServer(t, pageConfig(),
		[]platform.Session{{ID: "2", Username: "casey", Seat: "seat0", Type: "wayland", Locked: true}},
		nil, nil, nil)
	body := authGet(t, s, "/").Body.String()
	if !strings.Contains(body, "wayland") || !strings.Contains(body, "locked") {
		t.Errorf("the session column does not show what the backend reported:\n%s", body)
	}
}

// The accounts page shows the gap the parent has not been told about: a human account
// on the machine that nothing enforces.
func TestTheUsersPageShowsTheUnmanagedCount(t *testing.T) {
	accounts := []users.Account{
		{Username: "casey", UID: 1000, Home: "/home/casey", Shell: "/bin/bash"},
		{Username: "bruce", UID: 1001, Home: "/home/bruce", Shell: "/bin/bash"},
	}
	s := pageServer(t, pageConfig(), nil, accounts, nil, nil)
	body := authGet(t, s, "/users").Body.String()
	if !strings.Contains(body, "bruce") {
		t.Error("an account on the machine is missing from the accounts page")
	}
	if !strings.Contains(body, "1") || !strings.Contains(strings.ToLower(body), "not managed") {
		t.Errorf("the page does not count the unmanaged account:\n%s", body)
	}
}

// The profile dropdown shows the name the parent typed, not the id.
func TestTheUsersPageNamesProfilesTheWayTheParentDid(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	body := authGet(t, s, "/users").Body.String()
	if !strings.Contains(body, "School nights") {
		t.Errorf("the page shows the profile id rather than its name:\n%s", body)
	}
}

// Every page has a template, and every template has a route.
//
// The failure this pins is adding a handler and forgetting the .tmpl: the route then
// answers 500 with a line in the log, which is a bug report that takes an afternoon
// to read as a missing file.
func TestEveryContentRouteHasATemplate(t *testing.T) {
	got := strings.Join(pageNames(), ",")
	for _, want := range []string{"dashboard", "users"} {
		if !strings.Contains(got, want) {
			t.Errorf("no template named %q is embedded; found %q", want, got)
		}
	}
}

// The stylesheet is linked by its hashed URL.
//
// A plain /static/app.css would be served with the page's no-store, so the CSS would
// be re-fetched on every page view -- or, if it were cached instead, a fix would not
// appear on a parent's phone until the browser felt like revalidating. That second
// one is the change that gets reported as "you did not deploy it".
func TestTheStylesheetIsLinkedByItsHashedURL(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	body := authGet(t, s, "/").Body.String()
	if !strings.Contains(body, "/static/app.") || strings.Contains(body, "/static/app.css\"") {
		t.Errorf("the stylesheet is not linked by its content hash:\n%s", body)
	}
}

// The page is not cacheable.
func TestTheContentPagesAreNotCacheable(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	for _, path := range []string{"/", "/users"} {
		if got := authGet(t, s, path).Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("GET %s Cache-Control = %q, want no-store", path, got)
		}
	}
}

// A handler that names a template nobody embedded is a programming error, and the
// useful answer is a 500 plus a log line naming the page -- not a panic that the
// recovery middleware turns into an "internal error" with nothing saying which
// handler was missing what.
func TestRenderingAPageThatWasNeverEmbeddedSaysWhichOne(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	s.render(rec, httptest.NewRequest("GET", "/nope", nil), "nonexistent", struct{}{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// A profile with no name is possible in the document, and a dropdown that renders an
// empty entry is indistinguishable from a broken control.
func TestAnUnnamedProfileIsStillNamedSomething(t *testing.T) {
	c := pageConfig()
	c.Profiles[0].Name = ""
	s := pageServer(t, c, nil, nil, nil, nil)
	body := authGet(t, s, "/users").Body.String()
	if strings.Contains(body, "<option value=\"school\"></option>") || strings.Contains(body, "school</span>\n") {
		t.Error("the profile rendered as an empty label")
	}
	// The id is the fallback, so the row still says which profile is attached.
	if !strings.Contains(body, "school") {
		t.Errorf("an unnamed profile lost its identity entirely:\n%s", body)
	}
}

// The listing's notes have to arrive on the page, which is the reason the accounts seam
// carries a *users.Listing rather than its []Account.
//
// Before the seam was widened, internal/users could write as many notes as it liked and
// none of them could reach a browser: the seam's signature had nowhere to put them. A
// child excluded by a UID_MIN the operator never meant was simply absent from the table,
// which is the exact failure the package's own doc comment says it exists to prevent.
func TestTheAccountsPageRendersTheListingsNotes(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	s.Accounts = func() (*users.Listing, error) {
		return &users.Listing{
			Accounts: []users.Account{{Username: "casey", UID: 1000, GID: 1000, Home: "/home/casey", Shell: "/bin/bash"}},
			UIDMin:   1000,
			UIDMax:   60000,
			Notes:    []string{"the file sets no UID_MIN; assuming 1000, which hides any account below it"},
		}, nil
	}
	rec := authGet(t, s, "/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /users = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "the file sets no UID_MIN") {
		t.Error("the listing's note never reached the page; the seam is dropping it somewhere")
	}
	// The bounds are part of the same explanation: "where is my kid" is answered by the
	// numbers that were applied.
	if !strings.Contains(body, "1000") || !strings.Contains(body, "60000") {
		t.Error("the uid range that was actually applied is not shown")
	}
}

// A source that answers with no listing at all is a failure, not an empty machine.
func TestAnAccountSourceThatAnswersNothingIsRefused(t *testing.T) {
	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	s.Accounts = func() (*users.Listing, error) { return nil, nil }
	rec := authGet(t, s, "/users")
	if rec.Code == http.StatusOK {
		t.Fatal("a nil listing rendered as \"no accounts\", which is a claim about the machine that " +
			"a source returning nothing did not establish")
	}
}
