package web

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// This file is #75: two properties that hold because of how the mechanism is built, with
// nothing checking that the codebase still fits the mechanism.
//
// Both guards are written the same way on purpose: neither carries a list of the things it
// checks. The route guard enumerates the registrations out of the package's own source and
// the form guard enumerates the embedded templates, so a route or a form added next month
// is covered the moment it is written -- which is the only difference between a guard and a
// snapshot. A guard that re-lists what it is guarding can only ever agree with itself.

// Why the route guard reads source instead of walking the router.
//
// #75 proposed http.ServeMux.Walk, "Go 1.22+". It is not there. On this repo's toolchain
// (go1.27.2) neither the method nor the old package-level http.WalkMux exists:
//
//	undefined: http.WalkMux
//	type *http.ServeMux has no field or method Walk
//
// and grep over $(go env GOROOT)/src finds no WalkMux at all. So the router cannot be
// enumerated, and the alternative is the cmd/plock/help_flags_test.go precedent: a meta-test
// over declarations. That has a real blind spot -- a route registered through a value the
// scanner cannot read -- and the guard closes it by refusing to be quietly blind: a
// registration with a non-literal pattern is an error, and every pattern it does read has
// to be provably live in the router.

// reachableWithoutASession is every route this server answers to an anonymous caller, with
// the reason it may stay open. Anything registered and absent from here must refuse.
//
// It is deliberately short, and deliberately the only place the answer is written down. The
// server keeps two other copies of the same fact -- requireAuth wraps a handler at
// registration, and isPublic is the CSRF layer's path list -- and isPublic's own comment
// says two lists drifting is how a protected route stops demanding a token. The guard below
// compares all three, so the drift is a failing test rather than a production property
// nobody measured.
var reachableWithoutASession = map[string]string{
	"GET /healthz":       "the monitor's port check; it answers \"ok\" and nothing else",
	"GET /login":         "the sign-in page, which is exactly what somebody with no session needs",
	"POST /login":        "the sign-in form itself; a token binds to a session and this caller has none yet",
	"GET /static/{file}": "the content-hashed stylesheet and script; the login page links them, and there is nothing in a stylesheet to steal",
}

// pathValues says what to put in a registered pattern's path parameter so a request can
// actually be sent to it. A parameter the guard has no value for fails the test rather than
// quietly skipping the route: a skipped route is a route reported as covered.
var pathValues = map[string]string{
	"{name}": "casey",
	"{file}": "app.css",
	// The profile routes' id, taken from the shared fixture (pages_test.go builds the
	// profile "school" and the account "casey"). It has to be a profile that exists:
	// with an unknown id the request would be refused for the wrong reason and the
	// guard would report the auth layer as working on evidence that only proves the
	// lookup failed.
	"{id}": "school",
}

// routeDecl is one registration as the source states it.
type routeDecl struct {
	pattern string
	where   string // file:line, so a failure points at the line to change
}

// registeredRoutes reads every non-test file in this package and returns every mux
// registration it finds.
//
// The receiver's name is not checked, because checking it would be the blind spot wearing
// a disguise: a guard that only looks at a variable named mux is indistinguishable from one
// that looks at nothing, the moment somebody writes r := http.NewServeMux(). Instead every
// .Handle / .HandleFunc call is collected and then proved against the live router below, so
// a false positive is a loud failure and not a silent pass.
func registeredRoutes(t *testing.T) []routeDecl {
	t.Helper()
	fset := token.NewFileSet()
	dir, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("the package cannot be parsed: %v", err)
	}
	if len(dir) == 0 {
		t.Fatal("the scan parsed no packages, so it is checking nothing")
	}

	var out []routeDecl
	for _, pkg := range dir {
		for name, file := range pkg.Files {
			decls, problems := collectRegistrations(fset, file, name)
			for _, problem := range problems {
				t.Error(problem)
			}
			out = append(out, decls...)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].pattern < out[j].pattern })
	return out
}

// collectRegistrations reads one parsed file and returns every mux registration in it, plus
// anything at a registration site the guard could not make sense of.
//
// It reports problems instead of failing the test because the guard's own logic needs to be
// testable against sources the real package does not contain. A branch that only fires on a
// shape nobody has today is untestable in situ, and an untestable branch is one a future
// edit can delete without anything noticing -- which is how a guard quietly stops being a
// guard. cmd.json's CH series guards help_flags_test.go for the same reason.
func collectRegistrations(fset *token.FileSet, file *ast.File, name string) ([]routeDecl, []string) {
	var out []routeDecl
	var problems []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc" {
			return true
		}
		line := fset.Position(call.Pos()).Line
		if len(call.Args) == 0 {
			problems = append(problems, fmt.Sprintf("%s:%d: a Handle call with no pattern argument", name, line))
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			// This is the guard's one real blind spot, so it is an error and not a comment: a
			// pattern built at run time is invisible to a source scan, and the scan would
			// otherwise keep reporting full coverage.
			problems = append(problems, fmt.Sprintf(
				"%s:%d registers a route with a pattern that is not a string literal. This guard "+
					"reads source and cannot see it -- either inline the literal or say there why "+
					"the route set is now dynamic.", name, line))
			return true
		}
		out = append(out, routeDecl{
			pattern: strings.Trim(lit.Value, `"`),
			where:   name + ":" + strconv.Itoa(line),
		})
		return true
	})
	return out, problems
}

// The scanner is a guard, so it gets guarded. These are the shapes the live package cannot
// produce, which is exactly why they need a source of their own: a branch nothing exercises
// is a branch that can be deleted, and the deletion would look like a passing suite.
func TestTheRouteScanSeesWhatAServerCouldWrite(t *testing.T) {
	cases := []struct {
		name        string
		src         string
		wantPattern string // "" when nothing should be collected
		wantProblem string // substring; "" when no problem should be reported
	}{
		{
			name:        "a literal pattern is collected",
			src:         `package web` + "\n\n" + `func f() { mux.Handle("GET /x", nil) }`,
			wantPattern: "GET /x",
		},
		{
			name:        "HandleFunc counts the same as Handle",
			src:         `package web` + "\n\n" + `func f() { mux.HandleFunc("POST /y", nil) }`,
			wantPattern: "POST /y",
		},
		{
			name:        "a computed pattern is refused, not skipped",
			src:         `package web` + "\n\n" + `func f() { mux.HandleFunc(healthzPattern, nil) }`,
			wantProblem: "not a string literal",
		},
		{
			name:        "a call that is not a registration is ignored",
			src:         `package web` + "\n\n" + `func f() { _ = mux.Handler(r) }`,
			wantPattern: "",
		},
		{
			name:        "a registration with no arguments is reported",
			src:         `package web` + "\n\n" + `func f() { mux.Handle() }`,
			wantProblem: "no pattern argument",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "synthetic.go", tc.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("the synthetic source does not parse: %v", err)
			}
			decls, problems := collectRegistrations(fset, f, "synthetic.go")
			switch {
			case tc.wantPattern != "" && (len(decls) != 1 || decls[0].pattern != tc.wantPattern):
				t.Errorf("collected %v, want the one pattern %q", decls, tc.wantPattern)
			case tc.wantPattern == "" && len(decls) != 0:
				t.Errorf("collected %v, want nothing", decls)
			}
			joined := strings.Join(problems, "\n")
			if tc.wantProblem == "" && joined != "" {
				t.Errorf("reported problems nobody asked for: %s", joined)
			}
			if tc.wantProblem != "" && !strings.Contains(joined, tc.wantProblem) {
				t.Errorf("problems = %q, want one containing %q. A blind spot the guard stays "+
					"quiet about is a blind spot that reports full coverage.", joined, tc.wantProblem)
			}
		})
	}
}

// fieldIsTestOnly reports whether every write to a Server field happens in a _test.go
// file, and names every file that writes it. It is what licenses a source-based route
// guard at all, so it is the one scan in this file where a quiet answer is the worst
// possible answer.
func fieldIsTestOnly(t *testing.T, field string) (bool, []string) {
	t.Helper()
	fset := token.NewFileSet()
	// Deliberately includes the test files this time: the question is where writes live,
	// and excluding them is the whole answer.
	dir, err := parser.ParseDir(fset, ".", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("the package cannot be parsed: %v", err)
	}
	var written []string
	for _, pkg := range dir {
		written = append(written, fieldWrites(pkg.Files, field)...)
	}
	sort.Strings(written)
	for _, f := range written {
		if !strings.HasSuffix(f, "_test.go") {
			return false, written
		}
	}
	return true, written
}

// fieldWrites names every file that writes a struct field.
//
// Two spellings write a field, and only one of them is an assignment:
//
//	s.extraRoutes = f         an AssignStmt with a SelectorExpr on the left
//	&Server{extraRoutes: f}   a CompositeLit key, which is not a statement at all
//
// The second is how a Server is built, so a scan that reads assignments alone reports the
// field as test-only while production sets it in a constructor -- and the guard that
// licenses the route scan is exactly where that answer must not be trusted. Both spellings
// are collected, and TestTheFieldScanSeesBothWaysOfWritingAField proves it against sources
// the real package does not contain.
//
// The scan is package-local. For extraRoutes that is a boundary and not a hole: the field
// is unexported, so no package outside this one can write it at all, and which file in this
// package does is precisely what this returns. What the licence therefore proves is WHO may
// add a route. It does not prove what the registered function does -- a route registered by
// a function defined in another package is invisible to registeredRoutes -- which is why the
// failure text below says "written", not "registered", and why a production write to this
// field needs a second look rather than only a passing guard.
func fieldWrites(files map[string]*ast.File, field string) []string {
	var written []string
	seen := map[string]bool{}
	report := func(name string) {
		if !seen[name] {
			seen[name] = true
			written = append(written, name)
		}
	}
	for name, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if ok && sel.Sel.Name == field {
						report(name)
					}
				}
			case *ast.CompositeLit:
				if !isServerLiteral(x) {
					return true
				}
				for _, elt := range x.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if ok && key.Name == field {
						report(name)
					}
				}
			}
			return true
		})
	}
	return written
}

// isServerLiteral reports whether a composite literal builds a Server.
//
// A literal with no written type counts as one. That is the inner element of something like
// []*Server{{extraRoutes: f}}, where the type is implied by the enclosing literal and the
// node alone does not say what it builds. Guessing "not a Server" would be the quiet answer
// again; guessing "Server" costs a loud test failure naming a file, which is the direction
// this file always picks.
func isServerLiteral(lit *ast.CompositeLit) bool {
	switch t := lit.Type.(type) {
	case nil:
		return true
	case *ast.Ident:
		return t.Name == "Server"
	case *ast.StarExpr:
		id, ok := t.X.(*ast.Ident)
		return ok && id.Name == "Server"
	case *ast.SelectorExpr:
		// web.Server, which is how the external test package spells it.
		return t.Sel.Name == "Server"
	}
	return false
}

// TestTheFieldScanSeesBothWaysOfWritingAField pins the licence the route guard runs under.
//
// The guard's own pass is not evidence here: extraRoutes is written only in test files
// today, so a scan that sees one of the two spellings still reports "test only" and stays
// green. Every case that must be caught is therefore a shape this package does not have,
// fed through the same function -- the collectRegistrations precedent, for the same reason:
// a branch nothing exercises is a branch that can be deleted, and the deletion looks like a
// passing suite.
func TestTheFieldScanSeesBothWaysOfWritingAField(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		src      string
		want     bool
	}{
		{
			name:     "an assignment in a production file",
			filename: "prod.go",
			src:      "package web\n\nfunc probe(s *Server) { s.extraRoutes = func(m any) {} }\n",
			want:     true,
		},
		{
			// The reviewer's probe: a composite literal, which is no kind of statement.
			name:     "a composite literal in a production file",
			filename: "prod.go",
			src:      "package web\n\nfunc probe() *Server { return &Server{extraRoutes: nil} }\n",
			want:     true,
		},
		{
			name:     "the same field written through a slice element, where the literal has no type",
			filename: "prod.go",
			src:      "package web\n\nfunc probe() { _ = []*Server{{extraRoutes: nil}} }\n",
			want:     true,
		},
		{
			name:     "the same literal in a test file, which is licensed",
			filename: "prod_test.go",
			src:      "package web\n\nfunc probe() *Server { return &Server{extraRoutes: nil} }\n",
			want:     false,
		},
		{
			// Control: the scan keys on the TYPE as well as the field name, or every struct
			// literal in the package with a same-named field would be reported.
			name:     "a different type's field of the same name",
			filename: "prod.go",
			src: "package web\n\ntype other struct{ extraRoutes int }\n\n" +
				"func probe() *other { return &other{extraRoutes: 1} }\n",
			want: false,
		},
		{
			// Control for the other direction: a read is not a write.
			name:     "reading the field is not writing it",
			filename: "prod.go",
			src: "package web\n\nfunc probe(s *Server) {\n\tif s.extraRoutes != nil {\n" +
				"\t\ts.extraRoutes(nil)\n\t}\n}\n",
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, tc.filename, tc.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("the synthetic source does not parse: %v", err)
			}
			written := fieldWrites(map[string]*ast.File{tc.filename: f}, "extraRoutes")
			inProduction := false
			for _, name := range written {
				if !strings.HasSuffix(name, "_test.go") {
					inProduction = true
				}
			}
			if inProduction != tc.want {
				t.Errorf("fieldWrites reported %v for %s, want it to %s a production write",
					written, tc.filename, map[bool]string{
						true: "find", false: "ignore",
					}[tc.want])
			}
		})
	}
}

// concretePath turns a registered pattern into a path a request can use.
func concretePath(t *testing.T, pattern string) string {
	t.Helper()
	_, p, _ := strings.Cut(pattern, " ")
	p = strings.TrimSuffix(p, "{$}") // "GET /{$}" is the root, not a path named "{$}"
	for placeholder, value := range pathValues {
		p = strings.ReplaceAll(p, placeholder, value)
	}
	if strings.ContainsAny(p, "{}") {
		t.Fatalf("%s has a path parameter this guard has no value for; add it to pathValues "+
			"rather than letting the request miss the route it is meant to be testing", pattern)
	}
	return p
}

func methodOf(pattern string) string {
	m, _, _ := strings.Cut(pattern, " ")
	return m
}

func TestEveryRegisteredRouteIsEitherListedOrRefused(t *testing.T) {
	decls := registeredRoutes(t)
	if len(decls) == 0 {
		t.Fatal("the scan found no route registrations at all, so this guard is checking nothing")
	}

	// Control, and the load-bearing one: the scan has to see a registration inside a
	// function body, not merely a top-level one, because every registration this server has
	// lives inside router(). A scan that only read the file's top level would report a
	// satisfying number of routes and see none of them.
	if !containsRoute(decls, "GET /healthz") {
		t.Fatal("the scan did not find GET /healthz, which is registered inside a function " +
			"body -- so the scan is not reading function bodies and every result below is " +
			"meaningless")
	}

	// The scan's blind spot has to be provably test-only, or the guard is coverage theatre:
	// extraRoutes is a real second way onto the router, and it is a field a production
	// caller could set.
	if onlyTests, files := fieldIsTestOnly(t, "extraRoutes"); !onlyTests {
		t.Errorf("extraRoutes is written outside the test files (%s), so routes can now be "+
			"registered at run time and this source scan can no longer see every route. #75's "+
			"guard needs a different mechanism, not a quieter test.", strings.Join(files, ", "))
	}

	s := pageServer(t, pageConfig(), nil, nil, nil, nil)
	mux := s.router()

	seen := map[string]bool{}
	for _, d := range decls {
		d := d
		t.Run(strings.ReplaceAll(d.pattern, " ", "_"), func(t *testing.T) {
			p := concretePath(t, d.pattern)
			reason, listed := reachableWithoutASession[d.pattern]

			// Liveness: the router itself has to say it registered that pattern. This is
			// what turns "the scanner read a string" into "the server has this route" -- a
			// registration behind a branch that never runs, or a pattern whose text drifted
			// from the table's, fails here instead of being reported as covered.
			req := httptest.NewRequest(methodOf(d.pattern), p, nil)
			if h, matched := mux.Handler(req); h == nil {
				t.Errorf("%s is registered in the source (%s) but the router has no handler "+
					"for %s, so the registration never runs", d.pattern, d.where, p)
			} else if matched != d.pattern {
				t.Errorf("%s is registered in the source (%s) but a request to %s matches %q "+
					"instead, so the route the source declares is not the route that answers",
					d.pattern, d.where, p, matched)
			}

			// Behaviour, through the whole chain rather than the bare mux.
			browser := httptest.NewRequest(methodOf(d.pattern), p, nil)
			// A browser's Accept, because the refusal looks different to a program: 303 to
			// /login for a browser, 401 as JSON for anything else. A parent's browser is what
			// the gate exists for.
			browser.Header.Set("Accept", "text/html")
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, browser)

			refused := rec.Code == http.StatusSeeOther && rec.Header().Get("Location") == "/login"
			switch {
			case listed && refused:
				t.Errorf("%s is listed as reachable without a session (%s), but the server "+
					"redirected an anonymous caller to /login. The list and the gate disagree.",
					d.pattern, reason)
			case !listed && !refused:
				t.Errorf("%s answered %d to an anonymous caller and is not in "+
					"reachableWithoutASession. Either put it behind requireAuth or give it a "+
					"row there saying why it may stay open.", d.pattern, rec.Code)
			}

			// The third copy of the same fact. isPublic decides whether the CSRF layer
			// demands a token; requireAuth decides whether a session is needed. They may be
			// expressed differently and may not disagree.
			if got, want := isPublic(p), listed; got != want {
				t.Errorf("isPublic(%q) = %v but %s is registered as reachable without a "+
					"session: %v. These are the two lists isPublic's own comment warns about.",
					p, got, d.pattern, want)
			}
			seen[d.pattern] = true
		})
	}

	// The other direction of drift: a row for a route that no longer exists keeps a
	// public-looking claim on the books after the route is gone, and the next person reads
	// it as a decision that still stands.
	for pattern := range reachableWithoutASession {
		if !seen[pattern] {
			t.Errorf("reachableWithoutASession still lists %s, which nothing registers any "+
				"more. Delete the row, or register the route again.", pattern)
		}
	}
}

func containsRoute(decls []routeDecl, want string) bool {
	for _, d := range decls {
		if d.pattern == want {
			return true
		}
	}
	return false
}

// formOnPage is one post form as a browser sees it.
type formOnPage struct {
	action   string
	hasToken bool
}

var (
	formBlockRe = regexp.MustCompile(`(?is)<form\b[^>]*>(.*?)</form>`)
	formTagRe   = regexp.MustCompile(`(?is)<form\b[^>]*>`)
	methodAttr  = regexp.MustCompile(`(?is)\bmethod\s*=\s*"?([a-zA-Z]+)`)
	actionAttr  = regexp.MustCompile(`(?is)\baction\s*=\s*"?([^">\s]+)"?`)
)

// postForms reads the forms out of a rendered page. A form with no method attribute is a
// GET form and is not the thing under test, so it is skipped rather than counted.
func postForms(body string) []formOnPage {
	tokenField := regexp.MustCompile(`(?is)<input\b[^>]*\bname\s*=\s*"` + regexp.QuoteMeta(csrfField) + `"`)
	var out []formOnPage
	for _, block := range formBlockRe.FindAllStringSubmatch(body, -1) {
		tag := formTagRe.FindString(block[0])
		m := methodAttr.FindStringSubmatch(tag)
		if m == nil || !strings.EqualFold(m[1], "post") {
			continue
		}
		action := ""
		if a := actionAttr.FindStringSubmatch(tag); a != nil {
			action = a[1]
		}
		out = append(out, formOnPage{action: action, hasToken: tokenField.MatchString(block[1])})
	}
	return out
}

// templateForm is a post form as a template defines it, before any value is filled in.
type templateForm struct {
	template string
	// pattern is the action with every {{...}} expression replaced by "*", so a template
	// writing /users/{{$who}}/override can be matched against the /users/casey/override a
	// browser actually receives.
	pattern string
	raw     string
}

func templateForms(t *testing.T) []templateForm {
	t.Helper()
	names, err := fs.Glob(pageFS, "templates/*.tmpl")
	if err != nil {
		t.Fatalf("the embedded templates cannot be listed: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no templates are embedded, so this guard is checking nothing")
	}
	expr := regexp.MustCompile(`\{\{[^}]*\}\}`)
	var out []templateForm
	for _, name := range names {
		b, err := fs.ReadFile(pageFS, name)
		if err != nil {
			t.Fatalf("the embedded template %s cannot be read: %v", name, err)
		}
		for _, block := range formBlockRe.FindAllStringSubmatch(string(b), -1) {
			tag := formTagRe.FindString(block[0])
			m := methodAttr.FindStringSubmatch(tag)
			if m == nil || !strings.EqualFold(m[1], "post") {
				continue
			}
			a := actionAttr.FindStringSubmatch(tag)
			if a == nil {
				t.Errorf("%s posts a form with no action attribute; this guard has no way to "+
					"tie it back to a page, so name its page by giving it an action.", name)
				continue
			}
			out = append(out, templateForm{
				template: name,
				pattern:  expr.ReplaceAllString(a[1], "*"),
				raw:      a[1],
			})
		}
	}
	return out
}

// Two channels put a token into a rendered page, and a page can lose either one on its own.
//
// The shell's sign-out form reads pageData.CSRF, which render fills from the session. A page
// body's forms read the *view's* field, filled through csrfCarrier -- and that is not the
// same value as the shell's, because inside {{template "body" .Data}} the name $ is re-bound
// to the value passed to that template, not to the outer one. Measured, not recalled:
//
//	OUTER=$.CSRF -> shell        {{template "body" .Data}}
//	BODY-DOLLAR={{$.CSRF}} -> view
//
// That scoping is what makes #75's stated risk exactly right, and it is why the pointer in
// &dashboardPage{...} is load-bearing rather than a style choice: setCSRF has a pointer
// receiver, so rendering the value type leaves the field empty and every override button on
// the dashboard is refused with 403 while the page still looks fine. W104 pins that, and
// this test is what notices when a token goes missing for any other reason -- an empty
// lookup, an edited template line, a new page whose view never implements the interface.
//
// Reading the rendered markup is also the only way to cover the second half of #75's ask: a
// form in a template that no page renders is a form nobody ever looked at, and no assertion
// about fields or interfaces can see that.
//
// pageStates are the fixture states the pages are rendered in. Two, not one, because the
// dashboard swaps its override form depending on whether an override is active: with a
// single state half the forms in dashboard.tmpl never render, and a guard that never sees a
// form cannot check it.
var pageStates = []struct {
	name   string
	mutate func(*config.Config)
}{
	{"no override", func(*config.Config) {}},
	{"override active", func(c *config.Config) {
		c.Overrides = []config.Override{{
			Username: "casey",
			Reason:   "film night",
			By:       "parent",
			Until:    insideWindow.Add(10 * time.Minute),
		}}
	}},
}

func TestEveryFormOnEveryPageCarriesTheToken(t *testing.T) {
	// Every post form the templates define, and every one a browser is actually served.
	// The first set is the reach check, the second is the property.
	defined := templateForms(t)
	if len(defined) == 0 {
		t.Fatal("no template defines a post form, so this guard is checking nothing")
	}
	// action -> where it was rendered, for the failure messages.
	rendered := map[string]string{}
	examined := 0

	for _, state := range pageStates {
		cfg := pageConfig()
		state.mutate(cfg)
		s := pageServer(t, cfg, nil, nil, nil, nil)
		// The trail seam. pageServer wires the three seams the content pages were
		// built with; the audit page added a fourth, and without it /audit answers
		// 503 -- and a page that never rendered is a page this walk cannot ask what
		// forms it carries. A fixture that enumerates every route has to carry every
		// seam serve wires, because the one route it cannot render is exactly the one
		// that would ship unguarded.
		s.Trail = &fakeTrail{tail: audit.Tail{Records: trailFixture(), FilesRead: 1}}

		for _, d := range registeredRoutes(t) {
			if methodOf(d.pattern) != "GET" {
				continue
			}
			if _, public := reachableWithoutASession[d.pattern]; public {
				continue // the login page's form carries no token by design, and says why
			}
			p := concretePath(t, d.pattern)
			rec := authGet(t, s, p)
			if rec.Code != http.StatusOK {
				t.Errorf("%s answered %d in state %q. A page that does not render cannot be "+
					"asked what forms it carries, so this is a failure and not a skip.",
					p, rec.Code, state.name)
				continue
			}
			for _, f := range postForms(rec.Body.String()) {
				examined++
				rendered[f.action] = "state " + state.name + " on " + p
				if !f.hasToken {
					t.Errorf("the form posting to %q on %s (%s) carries no %q field. The CSRF "+
						"gate refuses it with 403, so a parent presses the button, nothing "+
						"happens, and nothing says why. Two independent channels put a token in "+
						"a page -- pageData.CSRF for the shell, and the view's own field through "+
						"csrfCarrier for a page body -- and the rendered markup is the only place "+
						"either one is observable, which is why this reads the page rather than "+
						"the fields.",
						f.action, p, state.name, csrfField)
				}
			}
		}
	}

	if examined == 0 {
		t.Fatal("no post form was rendered by any page, so this guard checked nothing. The " +
			"shell has always carried the sign-out form; if that changed, say so here.")
	}

	// Reach, in the direction that matters: a form defined in a template that no walked page
	// ever rendered would have been skipped above, silently and forever.
	for _, d := range defined {
		if _, ok := rendered[d.raw]; ok {
			continue
		}
		var matched string
		for action, where := range rendered {
			if ok, err := path.Match(d.pattern, action); err == nil && ok {
				matched = where
				break
			}
		}
		if matched == "" {
			t.Errorf("%s defines a form posting to %s and no page this server registers ever "+
				"served it. Either it is dead markup or it belongs to a route this guard did "+
				"not reach; both mean the token check above never looked at it.",
				d.template, d.raw)
		}
	}
}
