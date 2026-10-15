package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docs/security.md is the document the release checklist is written against, and its rule is that
// every "we defend against this" claim has a test behind it. That rule is only real if something
// checks it, because the failure mode is silent and one-directional: a test gets renamed or
// deleted, the doc keeps citing the old name, and the file goes on asserting coverage that no
// longer exists while looking exactly as authoritative as it did before.
//
// So: every TestName this file cites in backticks must be a function that actually exists in the
// tree.

var (
	citedTest    = regexp.MustCompile("`([A-Z][A-Za-z0-9_]+)`")
	declaredTest = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)
)

func TestSecurityDocCitesOnlyTestsThatExist(t *testing.T) {
	doc := readRepoFile(t, "docs/security.md")

	root := repoRoot(t)

	// Collected from source rather than from `go test -list`: a test in one package cannot ask
	// the go command what the other packages contain without shelling out, and the declaration
	// is the fact we actually care about -- that the name is a test someone wrote.
	declared := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range declaredTest.FindAllStringSubmatch(string(src), -1) {
			declared[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(declared) < 100 {
		// The walk finding almost nothing means it walked the wrong place, and a vacuous pass
		// here is worse than no check at all.
		t.Fatalf("found only %d test functions; the walk is not looking at the tree", len(declared))
	}

	var missing []string
	seen := map[string]bool{}
	for _, m := range citedTest.FindAllStringSubmatch(doc, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		// Only names that look like tests are checked; the same backtick shape is used for
		// identifiers, flags, and paths elsewhere in the file.
		if !strings.HasPrefix(name, "Test") {
			continue
		}
		if !declared[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		t.Errorf("docs/security.md cites %d test(s) that do not exist:\n  %s\n"+
			"Either restore the test, or if the coverage really is gone, say so in the file -- "+
			"a threat model that asserts a defense nobody wrote is worse than one that admits the gap.",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// The document is the release checklist's basis for "every claim has a test", so it must not be
// empty or have been truncated to a stub.
func TestSecurityDocMakesClaims(t *testing.T) {
	doc := readRepoFile(t, "docs/security.md")
	n := 0
	for _, m := range citedTest.FindAllStringSubmatch(doc, -1) {
		if strings.HasPrefix(m[1], "Test") {
			n++
		}
	}
	if n < 40 {
		t.Errorf("docs/security.md cites %d tests, want at least 40; the file has probably been replaced by a stub", n)
	}
	for _, section := range []string{"## 8. Not defended", "## 9. Residual risk"} {
		if !strings.Contains(doc, section) {
			t.Errorf("docs/security.md is missing %q; a threat model with no list of what it does NOT defend is not a threat model", section)
		}
	}
}

// repoRoot climbs to the module root. Tests run with their own package directory as the working
// directory, so a check that spans the tree has to start from the root rather than from "." --
// starting from "." is how this test initially walked only cmd/plock and found 54 of the tree's
// 473 test functions, which the premise guard below caught.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}

func readRepoFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
