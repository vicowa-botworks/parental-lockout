package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/version"
)

// The README and the first-run walk in docs/setup.md are what an operator reads before they
// have a binary to run, so nothing checked them: docs/cli.md has doc_test.go, and nothing else
// did. Writing the operator sections from memory of the commands produced five false claims,
// each of which read as though it had been measured:
//
//   - `plock version` was said to print "the toolchain it was built with". It prints a build
//     date; there is no toolchain in the output, and `make verify`'s own stamp gate asserts the
//     three-field line. docs/cli.md said the same thing and the README copied it.
//   - the argument-order paragraph claimed the wrong order "silently parses zero flags". The
//     name is lifted out before parsing, so it does not; the real trap is an unquoted value.
//   - `/settings` was said to hold the listen address. settings.tmpl says, in words, that the
//     listening address is NOT on that page.
//   - `/profiles` was said to toggle rules. No route and no form toggles a rule.
//   - docs/setup.md quoted `rules[3]` for a sequence that produces `rules[2]`, and wrapped a
//     one-line refusal over four.
//
// Every guard below therefore reads the artifact the claim is about — the command list, the
// router, the version string, the command's own stderr — rather than re-reading the prose. A
// check that re-read the prose would have agreed with the prose that was wrong.

// repoFile reads a file at the repository root, from cmd/plock.
func repoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// TestReadmeCommandTableNamesEveryCommand pins the README's command table to `plock -h`.
//
// The table is the README's own claim about the surface ("Every one of them does something;
// none is a stub"), so it has to be the same set the binary announces. A command added without
// a row leaves the entry point describing half the product; a row left behind after a rename
// sends an operator to a command that answers "unknown command".
func TestReadmeCommandTableNamesEveryCommand(t *testing.T) {
	readme := repoFile(t, "README.md")

	// The table is the indented `name<spaces>description` column inside a fenced block, the
	// same shape globalUsage uses. One space is prose ("plock unlock casey --minutes 20"),
	// two or more is a row.
	row := regexp.MustCompile(`^plock ([a-z][a-z-]+)\s{2,}\S`)
	var listed []string
	for _, line := range strings.Split(readme, "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			listed = append(listed, m[1])
		}
	}
	if len(listed) < 5 {
		t.Fatalf("only %d command rows parsed out of README.md; the table format changed and "+
			"this test is now checking nothing", len(listed))
	}

	want := topLevelCommands(t)
	got := append([]string(nil), listed...)
	sort.Strings(got)
	if strings.Join(got, ",") == strings.Join(want, ",") {
		return
	}

	missing := map[string]bool{}
	for _, c := range want {
		missing[c] = true
	}
	for _, c := range got {
		delete(missing, c)
	}
	for _, c := range got {
		if !contains(want, c) {
			t.Errorf("README.md lists `plock %s`; the binary does not have that command", c)
		}
	}
	for _, c := range want {
		if !contains(got, c) {
			t.Errorf("`plock %s` is announced by `plock -h`; README.md never gives it a row", c)
		}
	}
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

// TestReadmePageTableNamesRegisteredRoutes pins the README's page table to the router.
//
// The table is the only place the web app is described as a whole, and both directions have
// already gone wrong once: it offered a field the page refuses to have, and it omitted actions
// that were on the dashboard.
func TestReadmePageTableNamesRegisteredRoutes(t *testing.T) {
	readme := repoFile(t, "README.md")

	var pages []string
	inTable := false
	for _, line := range strings.Split(readme, "\n") {
		cells := splitRow(line)
		if len(cells) == 0 {
			inTable = false
			continue
		}
		if cells[0] == "page" {
			inTable = true
			continue
		}
		if !inTable || strings.HasPrefix(cells[0], "-") {
			continue
		}
		name := backticked(cells[0])
		if len(name) != 1 {
			t.Errorf("page table row names %d pages, want exactly 1: %q", len(name), line)
			continue
		}
		pages = append(pages, name[0])
	}
	if len(pages) < 4 {
		t.Fatalf("only %d page rows parsed out of README.md; the table format changed and this "+
			"test is now checking nothing", len(pages))
	}

	routes := registeredGETPages(t)
	for _, p := range pages {
		if !contains(routes, p) {
			t.Errorf("README.md's page table lists %q; the router registers no GET for it. "+
				"Routes are in internal/web/server.go", p)
		}
	}
	// The other direction: a page added to the router belongs in the README, or the entry
	// point quietly stops describing the product. /login and /healthz are excluded below.
	for _, r := range routes {
		if !contains(pages, r) {
			t.Errorf("the router serves GET %q; README.md's page table never mentions it", r)
		}
	}
}

// registeredGETPages reads internal/web/server.go and returns every GET page pattern, with
// net/http's `{$}` exact-match suffix removed so `/{$}` reads as the `/` an operator types.
//
// It parses the source rather than importing the package because the router keeps its patterns
// to itself — there is no exported list to read, and inventing one for a test would put a
// second source of truth where the first one already lives.
func registeredGETPages(t *testing.T) []string {
	t.Helper()
	src := repoFile(t, "internal", "web", "server.go")
	// Patterns are written as literals here; a hand-written config value could not add a route.
	//nolint:gosec
	pat := regexp.MustCompile(`"GET (/[^"]*)"`)
	hidden := map[string]bool{"/login": true, "/healthz": true, "/static/{file}": true}
	var out []string
	for _, m := range pat.FindAllStringSubmatch(src, -1) {
		p := strings.TrimSuffix(m[1], "{$}")
		if p == "" {
			p = "/"
		}
		if hidden[p] {
			continue
		}
		if !contains(out, p) {
			out = append(out, p)
		}
	}
	if len(out) < 4 {
		t.Fatalf("found %d GET pages in internal/web/server.go: %v — the registration format "+
			"changed and this check is now vacuous", len(out), out)
	}
	sort.Strings(out)
	return out
}

// TestCLIDocQuotesWhatVersionPrints pins the line docs/cli.md shows for `plock version`.
//
// The document used to say the command prints "the Go toolchain the binary was built with".
// internal/version has three fields — version, commit, build date — and none of them is a
// toolchain, so the sentence described a field that does not exist. Quoting the real line is
// the pin; the word scan is the belt, because a reader who never runs the command learns the
// field list from the prose and not from the sample.
func TestCLIDocQuotesWhatVersionPrints(t *testing.T) {
	doc := repoFile(t, "docs", "cli.md")

	want := "    " + version.String() + "\n"
	if !strings.Contains(doc, want) {
		t.Errorf("docs/cli.md does not quote the version line this build prints.\n"+
			"  want the line: %s\n  version.String() is the only source of that text; the "+
			"Makefile's stamp gate compares the same three fields.", strings.TrimSuffix(want, "\n"))
	}
	for _, word := range []string{"toolchain", "compiler", "runtime.Version"} {
		if strings.Contains(strings.ToLower(doc), strings.ToLower(word)) {
			t.Errorf("docs/cli.md mentions %q near `plock version`; the command prints version, "+
				"commit and build date, and nothing else", word)
		}
	}
}

// TestDocumentedConfigKeysExist pins every `settings.<name>` a document tells the operator to
// edit to a real field on config.Settings.
//
// It exists because the README now sends the operator to the config file for the one value the
// settings page refuses to hold, and a key that is not the key is worse than no instruction:
// the daemon ignores an unknown field, so the edit reads as done and the address never changes.
func TestDocumentedConfigKeysExist(t *testing.T) {
	tag := regexp.MustCompile("`settings\\.([a-zA-Z][a-zA-Z0-9_]*)`")
	tags := jsonTagsInSources(t)

	checked := 0
	for _, name := range []string{"README.md", "docs/cli.md", "docs/setup.md", "docs/security.md"} {
		for _, m := range tag.FindAllStringSubmatch(repoFile(t, strings.Split(name, "/")...), -1) {
			checked++
			if !tags[m[1]] {
				t.Errorf("%s tells the operator to set `settings.%s`; config.Settings has no such "+
					"field, so the daemon would read the file as if the line were absent", name, m[1])
			}
		}
	}
	if checked == 0 {
		t.Fatal("no `settings.<name>` appears in any document, so this check is vacuous — if the " +
			"README stopped naming config keys, delete this test rather than leaving it green")
	}
}

// TestSetupQuotedRefusalsAreRealOutput runs the sequence docs/setup.md shows and insists the
// quoted refusals are byte-for-byte what the commands wrote.
//
// The page promises, at its top: "Every command below was run while this page was written and
// its output is pasted from that run, so where the wording of an error matters you are reading
// the real wording." The quoted empty-window refusal named `rules[3]` for a sequence that
// produces `rules[2]`, and wrapped a one-line message over four — so the wording an operator
// was told to expect was not the wording they would get.
//
// It drives the commands rather than calling config.Validate because the index in the message
// is a property of the sequence: the same rule added after two rules or after three rules is a
// different sentence, and only the commands know which sequence the page shows.
func TestSetupQuotedRefusalsAreRealOutput(t *testing.T) {
	setup := repoFile(t, "docs", "setup.md")
	path := writeCfg(t, `{
  "version": 1,
  "settings": {"enforcement": true, "pollSeconds": 30, "lockBackends": ["logind"]},
  "profiles": [],
  "users": []
}`)

	// The four writes in §2, in the order the page prints them.
	for _, args := range [][]string{
		{"profile", "add", "schoolnight", "--name", "School night", "--timezone", "Europe/London"},
		{"profile", "rule", "add", "schoolnight", "bedtime", "--days", "mon,tue,wed,thu,sun",
			"--start", "21:00", "--end", "06:30", "--name", "Bedtime"},
		{"profile", "rule", "add", "schoolnight", "school", "--days", "mon,tue,wed,thu,fri",
			"--start", "08:15", "--end", "13:30", "--name", "School hours"},
		{"user", "assign", "casey", "schoolnight"},
	} {
		if code, out, errOut := runCmd(t, append(args, "--config", path)...); code != exitOK {
			t.Fatalf("setup.md runs `plock %s` and it exits %d\nstdout: %s\nstderr: %s",
				strings.Join(args, " "), code, out, errOut)
		}
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"profile rm", []string{"profile", "rm", "schoolnight"}},
		{"empty window", []string{"profile", "rule", "add", "schoolnight", "equal",
			"--days", "mon", "--start", "09:00", "--end", "09:00"}},
	} {
		code, _, errOut := runCmd(t, append(tc.args, "--config", path)...)
		if code == exitOK {
			t.Fatalf("setup.md quotes a refusal from `plock %s`; the command succeeded",
				strings.Join(tc.args, " "))
		}
		quoted := strings.TrimRight(errOut, "\n")
		if quoted == "" {
			t.Fatalf("`plock %s` refused on the command line but wrote nothing to stderr, so "+
				"there is nothing for the page to quote", strings.Join(tc.args, " "))
		}
		if !strings.Contains(setup, quoted) {
			t.Errorf("docs/setup.md does not quote what `plock %s` actually prints:\n%s\n"+
				"The page says its output is pasted from a real run, so paste this run.",
				strings.Join(tc.args, " "), quoted)
		}
	}
}
