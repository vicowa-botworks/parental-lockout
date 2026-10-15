package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// storedHasher is the cheapest cost a *stored* credential may legally claim.
//
// The authenticator is where the strength policy is enforced, so a test of it has
// to use a credential the policy would accept — otherwise every test below passes
// by falling through the strength error, which is exactly the wrong reason to be
// green. It costs about 40 ms a hash, which is the price of testing the real
// boundary.
func storedHasher() Hasher { return Hasher{Iterations: MinIterations} }

func TestAuthenticate(t *testing.T) {
	h := storedHasher()
	verifier, err := h.Hash("the administrator password")
	if err != nil {
		t.Fatal(err)
	}
	a := PasswordAuthenticator{Hasher: h, Username: "parent", Verifier: verifier}

	ok, err := a.Authenticate("parent", "the administrator password")
	if err != nil || !ok {
		t.Fatalf("correct credentials = (%v, %v)", ok, err)
	}
	if ok, err := a.Authenticate("parent", "wrong"); ok || err != nil {
		t.Errorf("wrong password = (%v, %v), want (false, nil)", ok, err)
	}
	if ok, err := a.Authenticate("root", "the administrator password"); ok || err != nil {
		t.Errorf("wrong username = (%v, %v), want (false, nil) — not an error, so the handler gives the same answer as a wrong password", ok, err)
	}
}

// Usernames are matched case-insensitively: a parent who created "parent" types
// "Parent" at 11pm. The fold is ASCII-only so it cannot disagree with however the
// config store happens to compare the same string elsewhere.
func TestTheUsernameIsFoldedButOnlyInASCII(t *testing.T) {
	h := storedHasher()
	verifier, _ := h.Hash("the administrator password")
	a := PasswordAuthenticator{Hasher: h, Username: "Parent", Verifier: verifier}

	for _, got := range []string{"parent", "Parent", "PARENT", "pArEnT"} {
		if ok, err := a.Authenticate(got, "the administrator password"); !ok || err != nil {
			t.Errorf("Authenticate(%q) = (%v, %v)", got, ok, err)
		}
	}
	// A non-ASCII case pair that Unicode folding would equate must not log in.
	unicode := PasswordAuthenticator{Hasher: h, Username: "straße", Verifier: verifier}
	if ok, _ := unicode.Authenticate("STRASSE", "the administrator password"); ok {
		t.Error("Unicode case folding equated two usernames the config store would treat as different")
	}
}

// A missing credential is an error, not a false.
//
// The two look identical from outside and are nothing alike to fix: one is
// somebody guessing, the other is a machine nobody can ever sign in to. A handler
// that cannot tell them will tell the parent "wrong password" while the real
// problem is that the account was never created.
func TestAMissingCredentialIsAnError(t *testing.T) {
	a := PasswordAuthenticator{Username: "parent"}
	ok, err := a.Authenticate("parent", "anything")
	if ok || !errors.Is(err, ErrNoCredential) {
		t.Errorf("= (%v, %v), want (false, ErrNoCredential)", ok, err)
	}
}

func TestABrokenCredentialIsAnErrorRatherThanADenial(t *testing.T) {
	a := PasswordAuthenticator{Username: "parent", Verifier: "not-a-verifier"}
	ok, err := a.Authenticate("parent", "anything")
	if ok {
		t.Fatal("a corrupt credential authenticated somebody")
	}
	if !errors.Is(err, ErrInvalidVerifier) {
		t.Errorf("err = %v, want ErrInvalidVerifier so the operator can be told", err)
	}
}

// An empty password must never be accepted, whatever the stored credential says.
//
// The case that matters is a config where the password field was cleared by hand
// or by a half-applied edit: an implementation that hashed the empty string and
// compared would find a match for a verifier of the empty string, and the machine
// would be open to anybody on the LAN.
func TestAnEmptyPasswordNeverAuthenticates(t *testing.T) {
	h := storedHasher()
	verifier, _ := h.Hash("the administrator password")
	a := PasswordAuthenticator{Hasher: h, Username: "parent", Verifier: verifier}
	if ok, _ := a.Authenticate("parent", ""); ok {
		t.Error("the empty password authenticated")
	}

	// And a verifier for an empty password cannot even be made.
	if _, err := h.Hash(""); !errors.Is(err, ErrEmptyPassword) {
		t.Errorf("Hash(\"\") = %v, want ErrEmptyPassword", err)
	}
}

// The username is checked after the password, and the password result is computed
// either way.
//
// This test is what caught a mutant the battery produced by reordering those two
// statements — and it only caught it after the ratio below was fixed. An
// assertion that can only fail in one direction is not an assertion.
//
// Short-circuiting on the username would answer in microseconds for a wrong one
// and a quarter of a second for the right one, which hands an attacker a free
// oracle for the half of the credential that is not secret — and turns the KDF's
// entire cost into a cost paid only on the interesting guess.
func TestTheUsernameIsNotAFastPath(t *testing.T) {
	h := storedHasher()
	verifier, _ := h.Hash("the administrator password")
	a := PasswordAuthenticator{Hasher: h, Username: "parent", Verifier: verifier}

	// Warm.
	a.Authenticate("parent", "the administrator password")

	const rounds = 10
	// Both guesses are wrong, and both are the same length; only the username
	// differs. If the password is verified either way the two take the same time.
	rightUser := measureAuth(a, "parent", "wrong password entirely", rounds)
	wrongUser := measureAuth(a, "nobody", "wrong password entirely", rounds)

	// The direction matters, and the first version of this assertion had it
	// backwards: it divided the slow path by the fast one and so passed happily
	// when the wrong-username path returned in microseconds. What must not happen
	// is the wrong username being *much quicker*, because that gap is the oracle.
	if float64(wrongUser)+1 < 0.6*(float64(rightUser)+1) {
		t.Errorf("a wrong username answered in %v against %v for the right one: the username short-circuits the KDF and is a timing oracle", wrongUser, rightUser)
	}
}

func measureAuth(a PasswordAuthenticator, user, pass string, rounds int) time.Duration {
	start := time.Now()
	for i := 0; i < rounds; i++ {
		a.Authenticate(user, pass)
	}
	return time.Since(start) / time.Duration(rounds)
}

// The interface is satisfied by the type, and by nothing else by accident.
func TestPasswordAuthenticatorSatisfiesTheInterface(t *testing.T) {
	var _ Authenticator = PasswordAuthenticator{}
	var _ Authenticator = rejectingAuthenticator{}
}

// rejectingAuthenticator is what a second implementation looks like, and it is
// here to pin the contract the interface documents: an implementation must not
// return true alongside an error.
type rejectingAuthenticator struct{}

func (rejectingAuthenticator) Authenticate(string, string) (bool, error) {
	return false, errors.New("pam is not implemented")
}

func TestTheInterfaceContractIsUsable(t *testing.T) {
	var a Authenticator = rejectingAuthenticator{}
	ok, err := a.Authenticate("parent", "whatever")
	if ok && err != nil {
		t.Errorf("an implementation returned (true, %v): the caller reads an error as \"no answer\"", err)
	}
}

// Two *valid* keys that agree on their first half must not verify against each
// other.
//
// The tail-edit test below cannot catch a comparison that only looks at the first
// 16 bytes: editing the last base64 character of a 32-byte key makes the field
// non-canonical, so the canonical check rejects it before the comparison runs,
// and the test passes for a reason that has nothing to do with what it claims to
// check. This one builds the near-miss key by hand so both encodings are clean
// and only the comparison can tell them apart.
func TestAKeyThatAgreesOnItsFirstHalfDoesNotVerify(t *testing.T) {
	h := testHasher()
	const password = "the administrator password"
	verifier, err := h.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(verifier)
	if err != nil {
		t.Fatal(err)
	}

	shortened := make([]byte, len(p.Key))
	copy(shortened, p.Key)
	shortened[len(shortened)-1] ^= 0xff
	nearMiss := fmt.Sprintf("%s$%d$%s$%s", scheme, p.Iterations,
		base64.RawStdEncoding.EncodeToString(p.Salt),
		base64.RawStdEncoding.EncodeToString(shortened))

	// The forged verifier must parse. If it does not, this test has degenerated
	// back into the tail-edit case and proves nothing about the comparison.
	if _, err := Parse(nearMiss); err != nil {
		t.Fatalf("the near-miss verifier should be well formed: %v", err)
	}
	if subtle.ConstantTimeCompare(p.Key[:16], shortened[:16]) != 1 {
		t.Fatal("the fixture does not share its first half; the test would pass vacuously")
	}
	if h.Verify(password, nearMiss) {
		t.Error("a key differing only in its second half verified: the comparison is truncated")
	}
}

// A verifier whose tail was edited, rather than truncated, must not verify.
//
// This is the case a lenient base64 decode loses: the last character of the key
// carries two significant bits, and a decoder that ignores them accepts a
// credential that is one edit away from the real one.
func TestAOneCharacterEditToTheVerifierDoesNotVerify(t *testing.T) {
	h := testHasher() // Verify has no strength policy; only the stored boundary does.
	verifier, _ := h.Hash("the administrator password")
	parts := strings.Split(verifier, "$")

	for i := range parts {
		b := []byte(parts[i])
		if b[len(b)-1] == 'A' {
			b[len(b)-1] = 'B'
		} else {
			b[len(b)-1] = 'A'
		}
		parts[i] = string(b)
		tampered := strings.Join(parts, "$")
		if h.Verify("the administrator password", tampered) {
			t.Errorf("a verifier with field %d edited still accepted the password", i)
		}
	}
}
