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
// visible. This test guards the cheap part: it fails on any loop or any equality
// comparison touching the key, which is what a prefix oracle looks like in the source.
//
// It reads the AST rather than the running code because the property is syntactic. There
// is no input that distinguishes constant-time from early-return except time, and time is
// the instrument that just failed.
func TestKeyComparisonIsConstantTime(t *testing.T) {
	fn := functionBody(t, "hash.go", "verifyWith")

	// A loop over the key is the oracle. There is no legitimate loop in this function: the
	// KDF call does the iterating and it lives inside crypto/pbkdf2.
	constantTime := false
	ast.Inspect(fn, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.ForStmt:
			t.Error("verifyWith has a loop; a byte-at-a-time key comparison is a per-byte oracle")
		case *ast.RangeStmt:
			t.Error("verifyWith ranges; a byte-at-a-time key comparison is a per-byte oracle")
		case *ast.BinaryExpr:
			if e.Op != token.EQL && e.Op != token.NEQ {
				return true
			}
			if mentionsKey(e.X) && mentionsKey(e.Y) {
				t.Errorf("verifyWith compares the key with %v; use subtle.ConstantTimeCompare", e.Op)
			}
		case *ast.CallExpr:
			if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ConstantTimeCompare" {
				constantTime = true
			}
		}
		return true
	})

	if !constantTime {
		t.Error("verifyWith never calls subtle.ConstantTimeCompare, so nothing here is constant time")
	}
}

// mentionsKey reports whether an expression reads a key -- the derived one or the stored
// one. `len(p.Key)` counts: an attacker who can time a length-dependent branch has a
// length oracle even if never a byte one.
func mentionsKey(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if id.Name == "key" || id.Name == "Key" {
				found = true
			}
		}
		return true
	})
	return found
}

// functionBody parses a file in this package and returns the named function's body.
func functionBody(t *testing.T, file, name string) *ast.BlockStmt {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
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
	raw, err := os.ReadFile("hash.go")
	if err != nil {
		t.Fatalf("read hash.go: %v", err)
	}
	fn := functionBody(t, "hash.go", "verifyWith")
	var compared string
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ConstantTimeCompare" {
			return true
		}
		parts := make([]string, 0, len(call.Args))
		for _, a := range call.Args {
			parts = append(parts, exprText(raw, a))
		}
		compared = strings.Join(parts, " vs ")
		return true
	})
	if compared == "" {
		t.Fatal("no ConstantTimeCompare call found")
	}
	// The two operands are the freshly derived key and the stored one -- not the verifier
	// string, not a re-encoded form of either.
	for _, want := range []string{"key", "p.Key"} {
		if !strings.Contains(compared, want) {
			t.Errorf("ConstantTimeCompare operands are %q; expected %q among them", compared, want)
		}
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
