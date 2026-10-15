package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every flag a command declares has to appear in that command's own help text, on the
// usage line or as the start of a line in the body.
//
// This is a test rather than a habit because `doctor` shipped its --json and --try flags
// with a usage line that said only `[--config PATH]`. The flags worked, `plock doctor -h`
// did not mention them, and nothing failed: the help text is a string, so an undocumented
// flag is invisible to every check except one that reads the two against each other.
//
// The check reads the source rather than the running FlagSet on purpose. A FlagSet only
// knows its flags once its command has been invoked, so a test built on FlagSets would
// have to call every command and would still miss a flag declared on a path the test did
// not reach. The AST sees all of them whether or not they are reachable.
//
// "On the usage line or at the start of a line" is what makes this mean something. A plain
// substring search passes when a flag is mentioned only in the prose of another flag's
// bullet -- which is exactly what happened while writing this test: `--try`'s description
// says "cannot be combined with --json", so a substring check reported --json documented
// after its own bullet had been deleted.
func TestEveryDeclaredFlagIsNamedInItsCommandsHelp(t *testing.T) {
	flagsByUsage := declaredFlags(t)
	texts := usageTexts(t)

	for usageConst, flags := range flagsByUsage {
		body, ok := texts[usageConst]
		if !ok {
			// Not every command binds its flags through a *Usage const (serve builds no
			// help topic of its own). Saying so is honest; failing would mean this test
			// owns a convention it cannot check.
			t.Logf("%s: no usage text found for the flags declared alongside it", usageConst)
			continue
		}
		for _, name := range flags {
			if !documented(body, name) {
				t.Errorf("--%s is declared but not documented in %s\n%s", name, usageConst, body)
			}
		}
	}
}

// documented reports whether the flag is presented as a flag: in the synopsis, or as the
// first thing on a line of the body.
//
// The synopsis is more than the first line. `profile`, `user` and `override` each list one
// invocation per line, and a flag that belongs to the second invocation belongs on the
// second line -- so treating only line 0 as the synopsis would demand that `profile rm`'s
// flags be crammed onto the `profile list` line.
//
// A prose mention does not count, and that is the whole point. While this test was being
// written, deleting `doctor`'s `--json` bullet left the check green, because `--try`'s
// bullet says "cannot be combined with --json". A flag someone has to guess the existence
// of is not documented.
func documented(usage, name string) bool {
	dash := "--" + name
	for i, l := range strings.Split(usage, "\n") {
		trimmed := strings.TrimSpace(l)
		isSynopsis := i == 0 ||
			strings.HasPrefix(trimmed, "usage:") ||
			strings.HasPrefix(trimmed, "plock ") ||
			strings.HasPrefix(trimmed, "Usage:")
		if isSynopsis && strings.Contains(l, dash) {
			return true
		}
		if strings.HasPrefix(trimmed, dash) {
			return true
		}
	}
	return false
}

// declaredFlags maps the usage const a command assigns to fs.Usage -> the flag names that
// command declares. Pairing them lexically (the *Usage identifier nearest each declaration)
// is what keeps `--all` attributed to lock and not to the file it happens to share with
// another command.
func declaredFlags(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, path := range sourceFiles(t) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			var usage string
			var names []string
			ast.Inspect(fn, func(n ast.Node) bool {
				switch e := n.(type) {
				case *ast.AssignStmt:
					// fs.Usage = func() { ... lockUsage ... }
					for i, rhs := range e.Rhs {
						if !strings.Contains(exprString(e.Lhs[i]), "Usage") {
							continue
						}
						if id := usageNameIn(rhs); id != "" {
							usage = id
						}
					}
				case *ast.CallExpr:
					sel, ok := e.Fun.(*ast.SelectorExpr)
					if !ok {
						break
					}
					if sel.Sel.Name != "Bool" && sel.Sel.Name != "String" {
						break
					}
					if len(e.Args) == 0 {
						break
					}
					if lit, ok := e.Args[0].(*ast.BasicLit); ok {
						names = append(names, strings.Trim(lit.Value, `"`))
					}
				}
				return true
			})
			if usage == "" || len(names) == 0 {
				continue
			}
			out[usage] = append(out[usage], names...)
		}
	}
	if len(out) == 0 {
		t.Fatal("no command declared any flag, so this test checked nothing")
	}
	return out
}

// usageTexts reads the back-quoted body of every `const xUsage = `...“ in the package.
func usageTexts(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
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
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
					continue
				}
				name := vs.Names[0].Name
				if !strings.HasSuffix(name, "Usage") {
					continue
				}
				lit, ok := vs.Values[0].(*ast.BasicLit)
				if !ok {
					continue
				}
				// The value is a raw string literal, so its Value includes the backticks.
				out[name] = strings.Trim(lit.Value, "`")
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("no *Usage const found, so this test checked nothing")
	}
	return out
}

func sourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			out = append(out, filepath.Join(".", n))
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatal("no non-test sources in the package directory")
	}
	return out
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.Ident:
		return v.Name
	}
	return ""
}

// usageNameIn finds the `xUsage` identifier anywhere inside an expression.
//
// It has to search the subtree rather than look at the top: every command assigns
// `fs.Usage = func() { flagOut.WriteString(lockUsage) }`, so the const is buried inside a
// closure and a shallow read of the right-hand side finds a FuncLit with no name at all.
// That is exactly how the first version of this test ended up reporting that no command
// declares any flag.
func usageNameIn(e ast.Expr) string {
	name := ""
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && strings.HasSuffix(id.Name, "Usage") && name == "" {
			name = id.Name
		}
		return true
	})
	return name
}
