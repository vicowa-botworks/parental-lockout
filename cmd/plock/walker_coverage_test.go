package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The flag-documentation test is only as good as the walker that feeds it, and the walker has
// already been blind twice in two rounds: first to `fs.Int` (an allowlist of flag kinds), then
// to `unlockUsage` (a const built by concatenation, so it never reached the text map). Both
// times the test went green while a flag it claimed to check was invisible, and both times the
// only evidence was a `t.Logf` line nobody reads.
//
// So this test does not check help text. It checks that the walker sees what is there, by
// counting the declarations a second, independent way -- a text scan for `fs.<Method>("…"` --
// and requiring the two views to agree. A walker that silently narrows is the failure mode,
// and a narrowing is exactly what a future edit looks like.

var flagDecl = regexp.MustCompile(`\bfs\.[A-Za-z]+\(\s*"([a-z][a-z0-9-]*)"`)

func TestTheFlagWalkerSeesEveryFlagThePackageDeclares(t *testing.T) {
	fromAST := map[string]bool{}
	fromText := map[string]bool{}

	for _, path := range sourceFiles(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, m := range flagDecl.FindAllStringSubmatch(string(raw), -1) {
			fromText[m[1]] = true
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, raw, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			for _, name := range flagNamesIn(fn) {
				fromAST[name] = true
			}
		}
	}

	if len(fromText) == 0 {
		t.Fatal("the text scan found no flag declarations, so this test checked nothing")
	}

	var missed []string
	for name := range fromText {
		if !fromAST[name] {
			missed = append(missed, name)
		}
	}
	sort.Strings(missed)
	if len(missed) > 0 {
		t.Errorf("the AST walker does not see %d flag(s) the package declares: %s\n\n"+
			"Every flag-documentation assertion silently skips a flag the walker misses, so a "+
			"help text can drop it and stay green. Either the declaration uses a form the "+
			"walker does not fold (a FlagSet bound through something other than "+
			"flag.NewFlagSet, or a name that is not a plain identifier), or the walker was "+
			"narrowed. Both are the bug this test is the guard for.",
			len(missed), strings.Join(missed, ", "))
	}

	// The reverse direction needs no assertion: the AST can only report a name that a
	// `fs.X("literal"` really contains, so a superset on that side is impossible by
	// construction rather than by luck.
}

// TestEveryDeclaredFlagIsAttributedToACommand closes the last way a flag can go unchecked.
//
// A flag the walker can see but cannot attribute to a usage const is skipped by the
// documentation test all the same: declaredFlags drops any function whose const it could not
// resolve. That is exactly what happens if newFlagSet's usage argument stops being read -- the
// profile and user flags are still collected, then thrown away, and the test reports itself
// green because it was never told to check them.
//
// So the union of what the pairing produces has to cover every flag in the package.
func TestEveryDeclaredFlagIsAttributedToACommand(t *testing.T) {
	attributed := map[string]bool{}
	for _, names := range declaredFlags(t) {
		for _, n := range names {
			attributed[n] = true
		}
	}

	var orphan []string
	for name := range declaredFlagNames(t) {
		if !attributed[name] {
			orphan = append(orphan, name)
		}
	}
	sort.Strings(orphan)
	if len(orphan) > 0 {
		t.Errorf("%d flag(s) are declared but attributed to no command's help text, so the "+
			"documentation test skips them: %s\n\n"+
			"A command whose usage const cannot be resolved is dropped by declaredFlags, which "+
			"is indistinguishable from a command that declares no flags. Reading the const out "+
			"of every binding form the package uses is what keeps that from being silent.",
			len(orphan), strings.Join(orphan, ", "))
	}
}

// declaredFlagNames is the package's full flag set, by text scan -- the same independent view
// the walker-coverage test uses, so this does not lean on the pairing to audit the pairing.
func declaredFlagNames(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, path := range sourceFiles(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, m := range flagDecl.FindAllStringSubmatch(string(raw), -1) {
			out[m[1]] = true
		}
	}
	if len(out) == 0 {
		t.Fatal("no flag declarations found, so the attribution check would pass vacuously")
	}
	return out
}

// TestEveryUsageConstAssignedToACommandHasReadableText is the same guard for the other half.
//
// A *Usage const the walker cannot read takes every flag beside it out of scope, which is how
// `unlockUsage` -- built with `+` -- hid for a full review round.
func TestEveryUsageConstAssignedToACommandHasReadableText(t *testing.T) {
	texts := usageTexts(t)

	assigned := map[string]bool{}
	for _, path := range sourceFiles(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, raw, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			// Which const this command binds its help to, read the two ways the package does
			// it. Matching any identifier ending in "Usage" instead swept in exitUsage (an
			// exit code) and the Usage field name, and reported them as unreadable help text.
			ast.Inspect(fn, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for i, rhs := range as.Rhs {
					if i < len(as.Lhs) && strings.Contains(exprString(as.Lhs[i]), "Usage") {
						if id := usageNameIn(rhs); id != "" {
							assigned[id] = true
						}
					}
					if id := usageArgIn(rhs); id != "" {
						assigned[id] = true
					}
				}
				return true
			})
		}
	}

	if len(assigned) == 0 {
		t.Fatal("no command references a *Usage const, so this test checked nothing")
	}
	var missing []string
	for name := range assigned {
		if _, ok := texts[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d *Usage const(s) are referenced by a command but have no readable text: %s -- "+
			"usageTexts is blind to them, so no flag beside them is ever checked",
			len(missing), strings.Join(missing, ", "))
	}
}
