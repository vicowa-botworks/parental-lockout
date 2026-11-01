package main

import (
	"bytes"
	"os"
	"os/exec"
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

// shOut runs a shell script from the repository root under `sh` and returns its `key=value` lines.
//
// `sh` and not `bash` because that is the shebang on every packaging script, and a guard written
// against a dialect the shipped code does not claim would pass on the way to a broken package.
// Every argument is one line-group of the script, joined with newlines, so a caller can pass the
// prologue and the probes as separate strings without tracking who owes a trailing newline.
func shOut(t *testing.T, script ...string) map[string]string {
	t.Helper()
	return shEnvOut(t, []string{"PKG_VERSION=v0.1.0"}, script...)
}

// shEnvOut is shOut with the environment stated rather than defaulted.
//
// PKG_VERSION is the door the helpers offer for saying what a build is, and it is the only way to
// ask about a tree this repository is not standing in. "Ten commits past the tag" would need a
// checkout ten commits past the tag -- a moving target that changes every time anything merges --
// while `PKG_VERSION=v0.1.0-10-geecf24a` produces, byte for byte, what that tree produces: measured
// at eecf24a with PKG_VERSION unset, `git describe` says v0.1.0-10-geecf24a and the helpers return
// deb 0.1.0+10.geecf24a and rpm 0.1.0-1.10.geecf24a, identical to what the same values return when
// handed to PKG_VERSION directly. One value of the door per column of the table.
func shEnvOut(t *testing.T, env []string, script ...string) map[string]string {
	t.Helper()
	joined := strings.Join(script, "\n")
	cmd := exec.Command("sh", "-c", joined)
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("sh -c failed: %v\nscript:\n%s\nstderr:\n%s", err, joined, stderr.String())
	}
	out := map[string]string{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			out[k] = v
		}
	}
	return out
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

// The three arms of docs/releases.md's version table, as column numbers.
//
// Named because a reader that takes a bare 2 or 3 cannot tell you which arm it meant when the
// table's column order changes, and the order is the page's argument: the middle column is the
// state a maintainer is usually in and the last is the rare one.
const (
	colTag   = 2 // HEAD is the tag
	colPast  = 3 // some commits past the tag; the tag is still reachable
	colNoTag = 4 // no tag reachable from HEAD at all
)

// armCol maps an arm's name in the test table to its column in the page.
func armCol(name string) int {
	switch name {
	case "past the tag":
		return colPast
	case "no tag reachable":
		return colNoTag
	}
	panic("unknown version arm " + name)
}

// versionCell reads one cell out of docs/releases.md's version table by row label and column.
//
// A missing row is fatal, not skipped: the whole point of reading a table is that a row can
// disappear, and a guard that returns "" when its row is gone agrees with the document that
// deleted it. The column argument is the same kind of guard: the table used to have two arms and
// a reader that always took cells[2] would have kept passing on a table that deleted the third
// column, which is the exact edit this file's own history makes impossible to remember. Column 1
// is the row label; 2, 3 and 4 are the three version arms.
func versionCell(t *testing.T, doc, label string, col int) string {
	t.Helper()
	prefix := "| " + label
	for _, line := range strings.Split(doc, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), prefix) {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) <= col {
			t.Fatalf("docs/releases.md row %q has %d cells and this guard wants column %d; the version "+
				"table lost an arm, and a reader that quietly returned the wrong one would agree with "+
				"the page that deleted it", label, len(cells)-2, col)
		}
		return strings.TrimSpace(cells[col])
	}
	t.Fatalf("docs/releases.md has no `%s` row in its version table. The row is the claim; if the "+
		"table changed shape, update this reader as well as the page.", label)
	return ""
}

// TestReleaseDocQuotesTheVersionsThePackagingProduces pins docs/releases.md's version table to
// scripts/package-lib.sh, which is what actually computes those strings.
//
// The page promises an operator what `dpkg-deb -f` and a filename will say about a release they
// are about to install on the machine that locks their children's screens. Written by hand it is
// a transcription of behaviour; the first release measured it correct (`Version: 0.1.0`,
// `plock-0.1.0-1.x86_64.rpm`, `plock v0.1.0 (commit …)`), and a measurement is not a pin. So the
// values here come from the helpers themselves, sourced the way scripts/package.sh sources them —
// `sh`, because that is the shebang on every packaging script and the one dialect this library
// claims to speak.
func TestReleaseDocQuotesTheVersionsThePackagingProduces(t *testing.T) {
	out := shOut(t, "set -eu\n. scripts/package-lib.sh\n"+
		`echo "raw=$(pkg_version)"`, `echo "deb=$(deb_version)"`,
		`echo "rpmver=$(rpm_version)"`, `echo "rpmrel=$(rpm_release)"`)

	doc := repoFile(t, "docs", "releases.md")
	tag := "v0.1.0" // any tagged shape; the point is the prefix, not the number

	for _, key := range []string{"raw", "deb", "rpmver", "rpmrel"} {
		if out[key] == "" {
			t.Fatalf("scripts/package-lib.sh printed no %s= value; the guard is reading nothing", key)
		}
	}

	// The other two arms of the table. Each is a `PKG_VERSION` the helpers can be handed, which is
	// how a table documents a tree the checkout is not in: "past the tag" is what `main` is now, and
	// it will be a different number of commits past it by the next merge, so the value is pinned to
	// the shape and the page quotes the instance.
	//
	//   past the tag        the state every ordinary checkout is in since the first tag landed.
	//                       `is_tagged` is TRUE here -- the tag is reachable -- so the version is
	//                       built ON the release and sorts above it.
	//   no tag reachable    a shallow clone, a fork without tags, a non-numeric PKG_VERSION. The
	//                       0.0.0 placeholder, which sorts below every release on purpose.
	arms := []struct {
		name string
		env  string
	}{
		{"past the tag", "PKG_VERSION=v0.1.0-10-geecf24a"},
		{"no tag reachable", "PKG_VERSION=sha-abc1234"},
	}
	for _, arm := range arms {
		arm := arm
		got := shEnvOut(t, []string{arm.env}, "set -eu\n. scripts/package-lib.sh\n"+
			`echo "raw=$(pkg_version)"`, `echo "deb=$(deb_version)"`,
			`echo "rpmver=$(rpm_version)"`, `echo "rpmrel=$(rpm_release)"`)
		for _, key := range []string{"raw", "deb", "rpmver", "rpmrel"} {
			if got[key] == "" {
				t.Fatalf("%s: scripts/package-lib.sh printed no %s= value", arm.name, key)
			}
		}
		// The two arms must not collapse into one another. Without this, a helper change that made
		// every untagged-ish string take the same branch would still satisfy the cell-by-cell
		// compares below whenever the page happened to agree with the collapsed value.
		if got["deb"] == out["deb"] {
			t.Errorf("%s: deb_version returned %q, the tagged release's own version. The whole point "+
				"of the arm is that a build is NOT the release it descends from.", arm.name, got["deb"])
		}
		if want := "`" + got["deb"] + "`"; versionCell(t, doc, "`.deb` version", armCol(arm.name)) != want {
			t.Errorf("docs/releases.md gives the .deb version for %s as %s; deb_version says %s. Update "+
				"the page from the helper, never the other way round.", arm.name,
				versionCell(t, doc, "`.deb` version", armCol(arm.name)), want)
		}
		if want := "`plock_" + got["deb"] + "_amd64.deb`"; versionCell(t, doc, "`.deb` file", armCol(arm.name)) != want {
			t.Errorf("docs/releases.md gives the .deb filename for %s as %s; scripts/package.sh writes "+
				"$OUT/plock_$(deb_version)_$(deb_arch <arch>).deb, so %s", arm.name,
				versionCell(t, doc, "`.deb` file", armCol(arm.name)), want)
		}
		want := "`plock-" + got["rpmver"] + "-" + got["rpmrel"] + ".<arch>.rpm`"
		if versionCell(t, doc, "`.rpm`", armCol(arm.name)) != want {
			t.Errorf("docs/releases.md gives the .rpm filename for %s as %s; rpm_version and rpm_release "+
				"say %s", arm.name, versionCell(t, doc, "`.rpm`", armCol(arm.name)), want)
		}
		// Every arm shows the parenthetical, because `version.String` prints it whatever it was
		// handed -- an unstamped binary says `(commit none, built unknown)` rather than dropping the
		// half that is missing. So the shape is the same in all three columns and only the version
		// token differs, which is what the tagged arm's own compare below already assumes.
		if claim := "`plock " + got["raw"] + " (commit"; !strings.Contains(versionCell(t, doc, "`plock version`", armCol(arm.name)), claim) {
			t.Errorf("docs/releases.md shows `plock version` for %s without the build stamp "+
				"(want %q in its cell); internal/version.String appends `(commit …, built …)` to every "+
				"arm, so a cell that stops at the version is describing an output no binary produces.",
				arm.name, claim)
		}
	}

	// The leading `v` is a tag prefix: pkg_version keeps it, the package versions do not. A
	// change to either half is a change an operator sees in a filename, so both halves are
	// asserted rather than the difference between them.
	if out["raw"] != tag {
		t.Fatalf("pkg_version with PKG_VERSION=%s returned %q; docs/releases.md is pinned to the "+
			"helpers, so this failure means the helper changed, not the page", tag, out["raw"])
	}
	if want := "`" + out["deb"] + "`"; versionCell(t, doc, "`.deb` version", colTag) != want {
		t.Errorf("docs/releases.md gives the .deb version as %s; deb_version says %s. Update the "+
			"page from the helper, never the other way round.", versionCell(t, doc, "`.deb` version", colTag), want)
	}
	if want := "`plock_" + out["deb"] + "_amd64.deb`"; versionCell(t, doc, "`.deb` file", colTag) != want {
		t.Errorf("docs/releases.md gives the .deb filename as %s; scripts/package.sh writes "+
			"$OUT/plock_$(deb_version)_$(deb_arch <arch>).deb, so %s",
			versionCell(t, doc, "`.deb` file", colTag), want)
	}
	if want := "`plock-" + out["rpmver"] + "-" + out["rpmrel"] + ".<arch>.rpm`"; versionCell(t, doc, "`.rpm`", colTag) != want {
		t.Errorf("docs/releases.md gives the .rpm filename as %s; rpm_version and rpm_release say %s",
			versionCell(t, doc, "`.rpm`", colTag), want)
	}
	if want := "`plock " + out["raw"] + " (commit"; !strings.Contains(versionCell(t, doc, "`plock version`", colTag), want) {
		t.Errorf("docs/releases.md does not show `plock version` keeping the tag prefix (want %q in its "+
			"row). The stamp gate in the Makefile compares the same string.", want)
	}
}

// TestReleaseDocNamesTheEventThatPublishes pins the page's central claim to the workflow: a tag
// push publishes, and nothing else does.
//
// This is the failure that produced the page being empty in the first place. README.md sends an
// operator to the releases page; the page is populated only by `.github/workflows/release.yml`,
// and the two conditions the document names — the `v*` trigger, and the job-level tag guard that
// keeps a manual dispatch from calling `gh release create` with a branch name — are both editable
// without touching a word of the prose. A workflow edited so that a merge could publish would make
// the document false in the direction that matters: a root daemon whose first act is to lock other
// people's screens, shipped without anybody deciding to ship it.
func TestReleaseDocNamesTheEventThatPublishes(t *testing.T) {
	wf := repoFile(t, ".github", "workflows", "release.yml")
	doc := repoFile(t, "docs", "releases.md")

	const (
		trigger = `- 'v*'`
		gate    = `if: startsWith(github.ref, 'refs/tags/')`
	)
	if !strings.Contains(wf, trigger) {
		t.Errorf("release.yml no longer triggers on a %s tag push; docs/releases.md still tells a "+
			"maintainer that pushing a tag is what publishes. One of the two is wrong, and the empty "+
			"releases page is how that stays hidden.", trigger)
	}
	// Scoped to the publish job, because the file contains the same condition twice and only one
	// of them is the claim. Measured: deleting the job-level `if:` and leaving the step-level one
	// passed a whole-file search — the step guard skips its own step, the artifacts job still goes
	// green, and the sibling publish job runs anyway. That is the failure the document describes,
	// so the reader has to be able to tell the two apart.
	start := strings.Index(wf, "\n  release:\n")
	if start < 0 {
		t.Fatalf("release.yml has no `release:` job; docs/releases.md's table names it, and this " +
			"guard can no longer tell which job publishes")
	}
	if !strings.Contains(wf[start:], gate) {
		t.Errorf("the release job in release.yml has no %s. Without it a workflow_dispatch calls "+
			"`gh release create` with GITHUB_REF_NAME set to the branch, and docs/releases.md says a "+
			"dispatch does not publish.", gate)
	}
	for _, claim := range []string{"`v*`", "refs/tags/"} {
		if !strings.Contains(doc, claim) {
			t.Errorf("docs/releases.md never states the trigger in terms a reader can grep for (%s), "+
				"so this guard's failure would not tell a maintainer which line of the workflow to look at.",
				claim)
		}
	}
}

// onBlock is the `on:` section of a workflow, read by indentation.
//
// Read by hand rather than parsed because this repository ships no third-party dependency --
// see scripts/check-deps.sh, which is part of `make verify` and refuses a `require` line in
// go.mod -- so a YAML library is not an option, and a hand-rolled reader is fifteen lines whose
// own behaviour the tests below establish. What it is for is a *set*: `strings.Contains(wf,
// "- 'v*'")` says the tag trigger still exists and says nothing about what else was added beside
// it, and the property the page claims in its first sentence is that nothing else publishes.
type onBlock struct {
	top      []string // top-level keys under `on:`
	pushKeys []string // keys directly under `push:`
	pushTags []string // list items under `push.tags:`
}

func readOnBlock(t *testing.T, wf string) onBlock {
	t.Helper()
	var b onBlock
	lines := strings.Split(wf, "\n")
	i := 0
	for ; i < len(lines); i++ {
		if lines[i] == "on:" {
			break
		}
	}
	if i == len(lines) {
		t.Fatalf("release.yml has no `on:` block at column 0; docs/releases.md's claim that only a tag " +
			"push publishes is unenforceable until this reader can find the triggers again")
	}
	inPush := false
	for _, l := range lines[i+1:] {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " "))
		switch {
		case indent == 0:
			return b // the next top-level key; `on:` is over
		case indent == 2:
			key := strings.TrimSuffix(trimmed, ":")
			inPush = key == "push"
			if strings.HasPrefix(trimmed, "- ") {
				key = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			}
			b.top = append(b.top, key)
		case indent > 2 && inPush:
			// Deeper than `push:` itself, at whatever depth the trigger's children sit: a key is a
			// trigger dimension (`branches`, `paths`, `tags`) and a list item is a value of the
			// dimension above it. Checked by depth rather than by counting on two-space steps,
			// because the file under test writes `- 'v*'` six columns in and a reader that assumed
			// four found nothing to compare.
			if strings.HasPrefix(trimmed, "- ") {
				b.pushTags = append(b.pushTags, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				continue
			}
			b.pushKeys = append(b.pushKeys, strings.TrimSuffix(trimmed, ":"))
		}
	}
	t.Fatalf("the `on:` block ran to the end of release.yml without a following top-level key")
	return b
}

// TestReleaseWorkflowPublishesOnTagsAndNothingElse pins the SET of things that can publish.
//
// The guard this replaces asked whether `- 'v*'` and the job-level `if:` were still in the file.
// Both survived every edit that matters: measured on this branch, adding `pull_request: branches:
// [main]` to `on:` left both tests green, and so did adding `branches: [main]` under `push` while
// keeping `tags` -- a merge to main would then create a release, and the page's first sentence says
// the opposite. A membership test cannot defend a set property, so this reads the block and asks
// the set question: which events, what under `push`, and which tags.
func TestReleaseWorkflowPublishesOnTagsAndNothingElse(t *testing.T) {
	wf := repoFile(t, ".github", "workflows", "release.yml")
	doc := repoFile(t, "docs", "releases.md")
	b := readOnBlock(t, wf)

	for _, key := range b.top {
		if key != "push" && key != "workflow_dispatch" {
			t.Errorf("release.yml now triggers on %q, and docs/releases.md opens by telling a maintainer "+
				"\"There is no other route\" than a tag push. `workflow_dispatch` is allowed because it "+
				"builds artifacts and the publish job's own `if:` refuses it; %q is not something that job "+
				"can refuse itself.", key, key)
		}
	}
	if len(b.top) == 0 {
		t.Fatal("the `on:` block has no triggers at all; nothing publishes, and the page needs rewriting")
	}
	// `branches` or `paths` under `push` is the merge-to-main publish. `tags` alone is the claim.
	for _, key := range b.pushKeys {
		if key != "tags" {
			t.Errorf("release.yml's `push` trigger carries %q beside `tags`. A branch push still fails the "+
				"publish job's `if: startsWith(github.ref, 'refs/tags/')`, so this does not publish by "+
				"itself -- what it does is move the whole protection onto one `if:` line, so that deleting "+
				"that line and merging to main together produce a release. docs/releases.md tells a reader "+
				"the trigger set is what prevents the accident; this guard is what keeps that sentence true "+
				"rather than accidentally true.", key)
		}
	}
	if len(b.pushTags) != 1 || b.pushTags[0] != "'v*'" {
		t.Errorf("release.yml publishes on push of %v; docs/releases.md names the pattern as `v*`, and a "+
			"second or different pattern means the page describes half the trigger set.", b.pushTags)
	}
	// The page has to keep saying the thing the guard defends, in words a reader can grep.
	for _, claim := range []string{"There is no other route", "`v*`", "refs/tags/"} {
		if !strings.Contains(doc, claim) {
			t.Errorf("docs/releases.md no longer states %q. This guard's failure has to point a reader at "+
				"the sentence that became false, and it cannot do that from a paraphrase.", claim)
		}
	}
}

// TestReleaseDocPinsTheWorkflowLinesItExplains covers three facts the page presents as measured
// history and nothing pinned at all: `GH_REPO` on the publish job, `mkdir -p staged` before the
// artifact count, and `--verify-tag`.
//
// Each is a line whose deletion the page says causes a specific failure, and each deletion was
// green. They are cheap, they are the kind of line a careless cleanup deletes ("the mkdir is
// redundant, download-artifact makes its path" -- it does not), and the page is now the canonical
// description of them, which is the definition of something worth a pin.
func TestReleaseDocPinsTheWorkflowLinesItExplains(t *testing.T) {
	wf := repoFile(t, ".github", "workflows", "release.yml")
	doc := repoFile(t, "docs", "releases.md")

	for _, pinned := range []struct {
		inWorkflow string
		inDoc      string
		why        string
	}{
		{"GH_REPO:", "`GH_REPO`",
			"without it `gh` resolves the repository from the runner's context and publishes somewhere " +
				"nobody chose"},
		{"mkdir -p staged", "`mkdir -p staged`",
			"download-artifact does not create its `path`, so the count that follows runs against a " +
				"directory that does not exist"},
		{"--verify-tag", "`--verify-tag`",
			"the release must name a commit the tag points at, not whatever GITHUB_REF_NAME resolved to"},
	} {
		if !strings.Contains(wf, pinned.inWorkflow) {
			t.Errorf("release.yml no longer contains %q, and docs/releases.md still explains it as one of "+
				"the guards that replaced an observed failure: %s.", pinned.inWorkflow, pinned.why)
		}
		if !strings.Contains(doc, pinned.inDoc) {
			t.Errorf("docs/releases.md no longer mentions %s. The workflow line %q is what the bullet "+
				"describes; if the guard is gone, delete the bullet in the same commit rather than "+
				"leaving a history lesson about a line that never existed.", pinned.inDoc, pinned.inWorkflow)
		}
	}
}
