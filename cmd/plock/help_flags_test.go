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
			// An error, not a log line. This branch is where `unlockUsage` hid: the const is
			// built by concatenation, so the walker never put it in the map, the flags
			// declared beside it were silently skipped, and the test still reported itself as
			// covering the package -- deleting --minutes from unlock's help stayed green.
			//
			// The original excuse ("not every command binds its flags through a *Usage const")
			// was never what this branch was for. declaredFlags only records a const that some
			// function actually assigns to fs.Usage, so a name in this map is by construction a
			// const that exists and is in use. If its text cannot be read, the walker is blind
			// and that is the failure this test exists to report.
			t.Errorf("%s is assigned to a command's fs.Usage and that command declares flags, "+
				"but usageTexts found no text for it -- the walker is blind to this const, so "+
				"every flag beside it is unchecked", usageConst)
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
	lines := strings.Split(usage, "\n")
	// The synopsis is the block before the first blank line, not merely the lines that begin
	// with `plock `. `profileUsage` wraps a long invocation:
	//
	//       plock profile rule add <id> <rule-id> --days LIST --start HH:MM --end HH:MM
	//                                       [--name TEXT] [--all-day] [--disable]
	//
	// and a rule keyed on the line prefix cannot see the continuation, so it reported two
	// documented flags as missing. Everything above the first blank line is an invocation by
	// construction in this package's help text, and prose only ever starts after it -- so
	// widening to the block costs nothing the prose rule was protecting.
	synopsisEnd := len(lines)
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			synopsisEnd = i
			break
		}
	}
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		isSynopsis := i < synopsisEnd ||
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
			names := flagNamesIn(fn)
			ast.Inspect(fn, func(n ast.Node) bool {
				// Only assignments are interesting here: this is the half that pairs a command
				// with its help const. The flag declarations themselves are collected by
				// flagNamesIn, which the coverage test calls too.
				if e, ok := n.(*ast.AssignStmt); ok {
					// fs.Usage = func() { ... lockUsage ... }
					for i, rhs := range e.Rhs {
						if !strings.Contains(exprString(e.Lhs[i]), "Usage") {
							continue
						}
						if id := usageNameIn(rhs); id != "" {
							usage = id
						}
					}
					// c := newFlagSet("profile add", profileUsage, false) -- the usage text is
					// an argument here, so a walker that only reads fs.Usage never learns which
					// const this command's help is, and the flags beside it have nothing to be
					// checked against.
					if usage == "" {
						for _, rhs := range e.Rhs {
							if id := usageArgIn(rhs); id != "" {
								usage = id
							}
						}
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
				// Not every *Usage const is a bare literal. unlockUsage is a BinaryExpr --
				// `…` + "`plock override clear <user>`" + `…` -- and requiring a BasicLit
				// here silently dropped it, which meant unlock's four flags were never
				// checked at all while the test still reported itself as covering the
				// package. Folding every string literal in the expression, in source order,
				// rebuilds the same text the compiler would.
				body := concatStrings(vs.Values[0])
				if body == "" {
					continue
				}
				out[name] = body
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

// flagNamesIn returns every flag name one command function declares.
//
// Both the documentation test and its coverage guard call THIS function, and that is the whole
// point of it being a function. The coverage test used to carry its own inline traversal, so
// restoring the Bool/String allowlist here left it green: a guard that re-implements the thing
// it guards cannot see that thing narrow.
//
// Deliberately NOT an allowlist of flag kinds. The first version named Bool and String, and
// `fs.Int("minutes", …)` in unlock.go sat outside it -- so deleting --minutes from unlock's
// help text stayed green, which is the same bug this test was written for. The rule is
// structural: any method on a FlagSet whose first argument is a string literal declares a
// flag. That covers String/Bool/Int and the Var forms today, and a kind added later is covered
// without anyone remembering to edit a list. Every other FlagSet method is excluded by
// construction -- Arg, Args, Name, NArg, Parse and SetOutput take no string literal first.
func flagNamesIn(fn *ast.FuncDecl) []string {
	var names []string
	// Which expressions in this function evaluate to a FlagSet, found by looking for the
	// assignment from flag.NewFlagSet or the package's own newFlagSet helper rather than by
	// matching the name `fs`, so a command that names its FlagSet something else is still
	// covered and a coincidentally-named variable is not.
	flagSets := flagSetVars(fn)
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !flagSets[exprString(sel.X)] {
			return true
		}
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			names = append(names, strings.Trim(lit.Value, `"`))
		}
		return true
	})
	return names
}

// flagSetVars returns the expressions in a function that evaluate to a FlagSet.
//
// Two idioms exist in this package and both have to be recognised, because a receiver that is
// not in this set is skipped and every flag behind it goes unchecked:
//
//	fs := flag.NewFlagSet(...)   -> the receiver is the identifier `fs`
//	c  := newFlagSet(...)        -> the receiver is the selector `c.fs`
//
// The second is manage.go's shared helper, which returns a cmdFlags struct holding the
// FlagSet in its fs field. Missing it is what left profile and user's eight flags outside the
// documentation check while the test reported itself as covering the package.
//
// The helper is matched by name rather than by return type because the AST has no type
// information here; if the helper is ever renamed, the coverage test
// TestTheFlagWalkerSeesEveryFlagThePackageDeclares fails rather than quietly narrowing.
func flagSetVars(fn *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range as.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}
			var key string
			switch exprString(call.Fun) {
			case "flag.NewFlagSet":
				key = ""
			case "newFlagSet":
				key = ".fs"
			default:
				continue
			}
			for _, lhs := range as.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					out[id.Name+key] = true
				}
			}
		}
		return true
	})
	return out
}

// usageArgIn returns the *Usage identifier passed as newFlagSet's second argument.
//
// Positional rather than searched, on purpose: newFlagSet's first argument is the command's
// name, which is a string literal and could be anything, and a subtree search over the whole
// call would also pick up the `usage` parameter name inside the helper's own body. Naming the
// slot is what keeps this from matching a call that has nothing to do with help text.
func usageArgIn(e ast.Expr) string {
	call, ok := e.(*ast.CallExpr)
	if !ok || exprString(call.Fun) != "newFlagSet" || len(call.Args) < 2 {
		return ""
	}
	if id, ok := call.Args[1].(*ast.Ident); ok && strings.HasSuffix(id.Name, "Usage") {
		return id.Name
	}
	return ""
}

// concatStrings rebuilds the text of a string expression that may be a concatenation of
// raw and interpreted literals.
//
// It only folds literals. An expression that calls a function or references another const
// has no statically known text, and returning "" says that honestly rather than guessing --
// the caller treats "" as "no usage text here", which is the same answer as a const the
// walker never saw.
func concatStrings(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return ""
		}
		// A raw literal's Value includes the backticks; an interpreted one's includes the
		// quotes and leaves escapes in. Every usage literal in this package is raw or plain
		// text with no escapes, so trimming the delimiters is enough to compare against a
		// flag name.
		return strings.Trim(v.Value, "`\"")
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return ""
		}
		return concatStrings(v.X) + concatStrings(v.Y)
	case *ast.ParenExpr:
		return concatStrings(v.X)
	}
	return ""
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
