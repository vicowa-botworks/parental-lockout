package auth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// The comparison in verifyWith has to be constant-time, and this asserts it structurally
// rather than by timing -- because the timing test cannot, and never could.
//
// Measured on this machine: pbkdf2.Key at the production floor costs 4.3 ms and
// subtle.ConstantTimeCompare over a 32-byte key costs 53 ns, which is 0.0012% of the
// operation. The timing test's floor is a ratio of 0.6, i.e. a 40% difference. No amount
// of sampling resolves a 0.0012% component against a 40% threshold, so replacing the
// constant-time compare with a per-byte early return -- the exact bug the test says it
// hunts -- passes it. Verified: an injected `for i := range key { if key[i] != p.Key[i] {
// return false } }` passed the timing test three times running, on both the mean and the
// minimum statistic.
//
// So the two tests divide the work by what each can actually see. The timing test guards
// the expensive part: a "fast path" that skips or caches the KDF moves 4.3 ms and is
// visible. This test guards the cheap part: it rejects any loop or any comparison that
// reads the same value the constant-time call is given.
//
// It reads the AST rather than the running code because the property is syntactic. There
// is no input that distinguishes constant-time from early-return except time, and time is
// the instrument that just failed.
//
// The rule follows the VALUE, not its spelling. An earlier version looked for the
// identifier "key" and required both operands to name it, which a local rename disarms:
// `if derived[0] == p.Key[0] && subtle.ConstantTimeCompare(derived, p.Key) == 1` is a
// one-bit oracle on the first byte of every failed login and that version called it clean.
// Anchoring on the operands of the ConstantTimeCompare call instead means the injected
// oracle is caught by the same rename that used to hide it -- the set of watched names is
// derived from the call, so it renames with it.
func TestKeyComparisonIsConstantTime(t *testing.T) {
	raw := sourceBytes(t, "hash.go")
	fn := functionBody(t, "hash.go", "verifyWith")

	// Exactly one constant-time call. Two would mean a second comparison somewhere that
	// this test's notion of "the key" has to be chosen between, and zero means the property
	// is not implemented -- an earlier version set a flag from any call anywhere in the
	// function, so one call satisfied it no matter what else the function did.
	calls := constantTimeCompares(fn)
	if len(calls) != 1 {
		t.Fatalf("verifyWith has %d ConstantTimeCompare calls, want exactly 1", len(calls))
	}
	call := calls[0]
	if len(call.Args) != 2 {
		t.Fatalf("ConstantTimeCompare takes two operands; this call has %d", len(call.Args))
	}
	watched := operandNames(raw, call)
	if len(watched) == 0 {
		t.Fatal("the constant-time call reads no named value, so there is nothing to watch")
	}

	ast.Inspect(fn, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.ForStmt:
			if readsAny(e, raw, watched) {
				t.Error("verifyWith loops over the compared value; a byte-at-a-time key comparison is a per-byte oracle")
			}
		case *ast.RangeStmt:
			if readsAny(e, raw, watched) {
				t.Error("verifyWith ranges over the compared value; a byte-at-a-time key comparison is a per-byte oracle")
			}
		case *ast.BinaryExpr:
			if !isComparison(e.Op) {
				return true
			}
			// The one legitimate comparison is the constant-time call's own result against
			// 1. Anything else that reads the compared value leaks a bit.
			if containsCall(e, call) {
				return true
			}
			if readsAny(e, raw, watched) {
				t.Errorf("verifyWith compares the compared value with %v outside the constant-time call; "+
					"that is an oracle on whatever the comparison reads first", e.Op)
			}
		}
		return true
	})
}

// constantTimeCompares returns every subtle.ConstantTimeCompare call in the subtree.
func constantTimeCompares(n ast.Node) []*ast.CallExpr {
	var out []*ast.CallExpr
	ast.Inspect(n, func(x ast.Node) bool {
		call, ok := x.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ConstantTimeCompare" {
			out = append(out, call)
		}
		return true
	})
	return out
}

// operandNames is the vocabulary this test watches: every value the constant-time call
// reads, by its source text.
//
// A selector is atomic on purpose. Descending into `p.Key` would add `p`, and `p` is how
// the receiver reaches everything -- p.Iterations, p.Salt -- so a set containing `p` flags
// the whole function and the test becomes noise nobody can act on.
func operandNames(raw []byte, call *ast.CallExpr) map[string]bool {
	names := map[string]bool{}
	for _, arg := range call.Args {
		ast.Inspect(arg, func(n ast.Node) bool {
			switch e := n.(type) {
			case *ast.SelectorExpr:
				names[exprText(raw, e)] = true
				return false
			case *ast.Ident:
				names[e.Name] = true
			}
			return true
		})
	}
	return names
}

// readsAny reports whether the subtree reads any of the watched values, using the same
// atomic-selector walk that built the set. Both sides have to agree or a selector on one
// side becomes a bare identifier on the other and nothing ever matches.
func readsAny(n ast.Node, raw []byte, watched map[string]bool) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if found {
			return false
		}
		switch e := x.(type) {
		case *ast.SelectorExpr:
			if watched[exprText(raw, e)] {
				found = true
			}
			return false
		case *ast.Ident:
			if watched[e.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

// containsCall reports whether the subtree is, or holds, the given call.
func containsCall(n ast.Node, call *ast.CallExpr) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if x == ast.Node(call) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func isComparison(op token.Token) bool {
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
		return true
	}
	return false
}

// sourceBytes reads a file in this package.
func sourceBytes(t *testing.T, file string) []byte {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return raw
}

// functionBody parses a file in this package and returns the named function's body.
func functionBody(t *testing.T, file, name string) *ast.BlockStmt {
	t.Helper()
	raw := sourceBytes(t, file)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, raw, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		if fn.Body == nil {
			t.Fatalf("%s has no body", name)
		}
		return fn.Body
	}
	t.Fatalf("no function %s in %s", name, file)
	return nil
}

// The stored encoding must not be what gets compared. Comparing the base64 strings would
// be constant-time in Go only by accident, and it would also mean a verifier that differs
// only in encoding compares unequal for the right reason and equal for none.
func TestVerifyComparesTheDerivedKeyNotItsEncoding(t *testing.T) {
	raw := sourceBytes(t, "hash.go")
	fn := functionBody(t, "hash.go", "verifyWith")

	calls := constantTimeCompares(fn)
	if len(calls) != 1 {
		t.Fatalf("verifyWith has %d ConstantTimeCompare calls, want exactly 1", len(calls))
	}
	parts := make([]string, 0, len(calls[0].Args))
	for _, a := range calls[0].Args {
		parts = append(parts, exprText(raw, a))
	}
	compared := strings.Join(parts, " vs ")
	// The two operands are the freshly derived key and the stored one -- not the verifier
	// string, not a re-encoded form of either.
	if !strings.Contains(compared, "Key") {
		t.Errorf("ConstantTimeCompare operands are %q; expected the stored key among them", compared)
	}
	if strings.Contains(compared, "base64") || strings.Contains(compared, "Encode") {
		t.Errorf("the comparison re-encodes something: %q", compared)
	}
}

// exprText renders an expression from the source bytes it came from. go/printer is not
// worth pulling in for two identifiers.
func exprText(raw []byte, e ast.Expr) string {
	start := int(e.Pos()) - 1
	end := int(e.End()) - 1
	if start < 0 || end > len(raw) || start >= end {
		return "?"
	}
	return strings.TrimSpace(string(raw[start:end]))
}
