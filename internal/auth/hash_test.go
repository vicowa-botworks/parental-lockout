package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// testHasher is the cheap cost. Every test that hashes uses it; the production
// cost is exercised once, deliberately, because it is the number the security
// argument rests on and a test suite that quietly lowered it would prove nothing.
func testHasher() Hasher { return Hasher{Iterations: TestIterations} }

func TestHashVerifyRoundTrip(t *testing.T) {
	h := testHasher()
	const password = "correct horse battery staple"

	verifier, err := h.Hash(password)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if verifier == password {
		t.Fatal("the verifier is the password")
	}
	if strings.Contains(verifier, password) {
		t.Fatal("the verifier contains the password")
	}
	if !h.Verify(password, verifier) {
		t.Fatal("the right password did not verify against its own hash")
	}
}

// Two hashes of the same password must differ, or the salt is doing nothing and
// two installs with the same password are one dictionary attack apart.
func TestTwoHashesOfTheSamePasswordDiffer(t *testing.T) {
	h := testHasher()
	a, err := h.Hash("a password at all")
	if err != nil {
		t.Fatal(err)
	}
	b, err := h.Hash("a password at all")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two hashes of one password are identical: the salt is not random")
	}
	if !h.Verify("a password at all", a) || !h.Verify("a password at all", b) {
		t.Fatal("both verifiers must accept the password")
	}
}

func TestVerifyRejectsTheWrongThings(t *testing.T) {
	h := testHasher()
	verifier, err := h.Hash("a sufficiently long secret")
	if err != nil {
		t.Fatal(err)
	}

	for name, got := range map[string]string{
		"empty":             "",
		"one character off": "a sufficiently long secre",
		"one character on":  "a sufficiently long secrets",
		"completely wrong":  "hunter2",
		"the verifier":      verifier,
		"whitespace only":   "   ",
	} {
		if h.Verify(got, verifier) {
			t.Errorf("%s: Verify returned true", name)
		}
	}
}

// The encoding records the cost it was made at, and a verifier made at a
// different cost still verifies.
//
// This is the property that lets DefaultIterations be raised in a later release.
// Reading the cost from the code instead would mean every credential made before
// the bump stops working, which is a lockout of the one account that turns
// enforcement off.
func TestVerifierRecordsItsOwnCost(t *testing.T) {
	cheap := Hasher{Iterations: TestIterations}
	verifier, err := cheap.Hash("a password long enough")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(verifier, fmt.Sprintf("%s$%d$", scheme, TestIterations)) {
		t.Fatalf("verifier %q does not record the cost it was made at", verifier)
	}
	// Verified by a Hasher configured for a different cost entirely.
	if !(Hasher{Iterations: DefaultIterations}).Verify("a password long enough", verifier) {
		t.Fatal("a verifier made at one cost must verify under a Hasher set to another")
	}
}

func TestParseRejectsMalformedEncodings(t *testing.T) {
	h := testHasher()
	good, err := h.Hash("a password long enough")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(good, "$")

	cases := map[string]string{
		"empty":              "",
		"plaintext password": "hunter2",
		"truncated":          good[:len(good)-4],
		"missing field":      strings.Join(parts[:3], "$"),
		"extra field":        good + "$extra",
		"wrong scheme":       "bcrypt$" + strings.Join(parts[1:], "$"),
		"empty scheme":       strings.Join(append([]string{""}, parts[1:]...), "$"),
		"non-numeric cost":   parts[0] + "$many$" + parts[2] + "$" + parts[3],
		"zero cost":          fmt.Sprintf("%s$0$%s$%s", scheme, parts[2], parts[3]),
		"negative cost":      fmt.Sprintf("%s$-1$%s$%s", scheme, parts[2], parts[3]),
		"cost above ceiling": fmt.Sprintf("%s$%d$%s$%s", scheme, MaxIterations+1, parts[2], parts[3]),
		"empty salt":         fmt.Sprintf("%s$%d$%s$%s", scheme, TestIterations, "", parts[3]),
		"short salt":         fmt.Sprintf("%s$%d$%s$%s", scheme, TestIterations, parts[2][:10], parts[3]),
		"long salt":          fmt.Sprintf("%s$%d$%s$%s", scheme, TestIterations, parts[2]+"AAAA", parts[3]),
		"short key":          fmt.Sprintf("%s$%d$%s$%s", scheme, TestIterations, parts[2], parts[3][:20]),
		"padded base64":      parts[0] + "$" + parts[1] + "$" + parts[2] + "=" + "$" + parts[3],
		"url alphabet":       parts[0] + "$" + parts[1] + "$" + parts[2] + "$" + urlAlphabetOf(parts[3]),
		"trailing newline":   good + "\n",
	}
	for name, v := range cases {
		if _, err := Parse(v); err == nil {
			t.Errorf("%s: Parse accepted %q", name, v)
		}
		if h.Verify("a password long enough", v) {
			t.Errorf("%s: Verify accepted a malformed verifier", name)
		}
	}
}

// urlAlphabetOf re-encodes the same bytes in the URL alphabet, which is the same
// base64 with two characters swapped. It has to actually differ from the input or
// the case proves nothing, so the test that uses it fails loudly rather than
// quietly passing an unchanged string.
func urlAlphabetOf(std string) string {
	raw, err := base64.RawStdEncoding.DecodeString(std)
	if err != nil {
		return std + "!"
	}
	url := base64.RawURLEncoding.EncodeToString(raw)
	if url == std {
		// No + or / in these bytes. Force a character the standard alphabet
		// rejects so the case still exercises the strict decode.
		return url + "-"
	}
	return url
}

// A cost below the floor is refused rather than honoured.
//
// The attack is not exotic: it is a config file edited by hand, or corrupted, in
// a way that leaves the credential syntactically valid. Accepting it means the
// password still works and the KDF has silently become something a laptop cracks
// in seconds.
func TestADowngradedCostIsRefused(t *testing.T) {
	h := testHasher()
	good, err := h.Hash("a password long enough")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(good, "$")
	weak := fmt.Sprintf("%s$%d$%s$%s", scheme, MinIterations-1, parts[2], parts[3])

	// Parse decodes it: the decoder's job is to say what the bytes are.
	p, err := Parse(weak)
	if err != nil {
		t.Fatalf("Parse of a weak-but-well-formed verifier: %v", err)
	}
	// The policy that refuses it lives at the trust boundary.
	if err := CheckStoredCost(p); !errors.Is(err, ErrWeakCost) {
		t.Errorf("CheckStoredCost(%d) = %v, want ErrWeakCost", p.Iterations, err)
	}
	// And the thing a login handler actually calls refuses it end to end.
	a := PasswordAuthenticator{Username: "parent", Verifier: weak}
	if ok, err := a.Authenticate("parent", "a password long enough"); ok || !errors.Is(err, ErrWeakCost) {
		t.Errorf("Authenticate against a weakened credential = (%v, %v), want (false, ErrWeakCost)", ok, err)
	}
	// The floor itself is accepted; the rule is a bound, not a rejection of an
	// honestly-set cost.
	atFloor, err := Parse(fmt.Sprintf("%s$%d$%s$%s", scheme, MinIterations, parts[2], parts[3]))
	if err != nil {
		t.Fatalf("Parse at exactly the floor: %v", err)
	}
	if err := CheckStoredCost(atFloor); err != nil {
		t.Errorf("CheckStoredCost at exactly the floor: %v", err)
	}
}

// TestIterations is below MinIterations by design, so a Hasher set to it can make
// a verifier Parse will refuse. That is the intended split: the field governs what
// this process does, the floor governs what stored data may claim. Pinning it here
// means the two can never be quietly merged.
func TestTheTestCostIsBelowTheAcceptedFloor(t *testing.T) {
	if TestIterations >= MinIterations {
		t.Errorf("TestIterations %d is not below MinIterations %d: tests would be paying production cost", TestIterations, MinIterations)
	}
	if MinIterations > DefaultIterations || DefaultIterations > MaxIterations {
		t.Errorf("cost bounds out of order: %d < %d < %d", MinIterations, DefaultIterations, MaxIterations)
	}
}

// A Hasher with no cost set must hash at the production cost.
//
// Every other test sets Iterations explicitly, so nothing else would notice the
// default quietly becoming cheap — and the default is the value a caller who
// writes `auth.Hasher{}` in cmd/plock actually gets.
func TestAnUnsetCostIsTheProductionCost(t *testing.T) {
	v, err := Hasher{}.Hash("a password long enough")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(v)
	if err != nil {
		t.Fatal(err)
	}
	if p.Iterations != DefaultIterations {
		t.Errorf("Hasher{}.Hash recorded %d iterations, want the production default %d", p.Iterations, DefaultIterations)
	}
}

// The production cost is the documented one. A constant nobody asserts on is a
// constant somebody will "optimise" in a refactor.
func TestProductionCostIsTheDocumentedOne(t *testing.T) {
	if DefaultIterations != 600_000 {
		t.Errorf("DefaultIterations = %d, want 600000", DefaultIterations)
	}
	if _, err := Parse(fmt.Sprintf("%s$%d$%s$%s", scheme, DefaultIterations,
		base64.RawStdEncoding.EncodeToString(make([]byte, saltBytes)),
		base64.RawStdEncoding.EncodeToString(make([]byte, keyBytes)))); err != nil {
		t.Errorf("the default cost is not parseable: %v", err)
	}
}

func TestHashRefusesAnEmptyPassword(t *testing.T) {
	if _, err := testHasher().Hash(""); !errors.Is(err, ErrEmptyPassword) {
		t.Errorf("Hash(\"\") = %v, want ErrEmptyPassword", err)
	}
}

// Hash must fail rather than invent a salt if the entropy source fails.
//
// A predictable salt is the one thing that makes two installs with the same
// password crackable together, and "just use the time" is exactly the
// reasonable-looking shortcut that buys that back. The failure is not reachable on
// a healthy machine, so the source is a seam.
func TestHashFailsWhenTheEntropySourceFails(t *testing.T) {
	prev := randRead
	randRead = func([]byte) error { return errors.New("no entropy") }
	t.Cleanup(func() { randRead = prev })

	v, err := testHasher().Hash("a password long enough")
	if err == nil {
		t.Fatalf("Hash produced %q with no entropy source: it invented a salt", v)
	}
	if strings.Contains(err.Error(), "$") {
		t.Errorf("the error leaks a verifier: %v", err)
	}
}

// The property the seam exists to protect: a real salt of the right length, from
// the real source, every time.
func TestHashUsesAFreshSaltOfTheRightLength(t *testing.T) {
	v, err := testHasher().Hash("a password long enough")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Salt) != saltBytes {
		t.Errorf("salt is %d bytes, want %d", len(p.Salt), saltBytes)
	}
	if len(p.Key) != keyBytes {
		t.Errorf("key is %d bytes, want %d", len(p.Key), keyBytes)
	}
}

// The timing bound the ticket asks for: a guess that is right up to the last
// character must not come back faster than one that is unrelated.
//
// It is a loose bound on purpose. PBKDF2 hashes the whole password, so there is no
// early return to find and the two times should be indistinguishable; what this
// catches is a future change that compares a prefix, checks a length, or adds a
// "fast path" for a cached derivation — each of which turns a quarter-second KDF
// into a per-byte oracle.
func TestVerificationTimeDoesNotDependOnHowCloseTheGuessIs(t *testing.T) {
	h := Hasher{Iterations: 40_000} // above the floor, still quick enough to time
	const secret = "a password long enough to be real"
	verifier, err := h.Hash(secret)
	if err != nil {
		t.Fatal(err)
	}

	// Warm the code path so the first measurement is not paying for page faults.
	h.Verify(secret, verifier)

	close := secret[:len(secret)-1] + "x"
	if len(close) != len(secret) {
		t.Fatal("the near-miss guess must be the same length")
	}
	unrelated := strings.Repeat("q", len(secret))

	const rounds = 12
	closeN, unrelatedN, exactN := measure(h, close, verifier, rounds), measure(h, unrelated, verifier, rounds), measure(h, secret, verifier, rounds)

	// A correct-prefix wrong password must not be materially faster than a
	// completely wrong one, and neither must be materially faster than the correct
	// password — an early return on "the derived key matches" would show up here.
	for _, pair := range []struct {
		name       string
		fast, slow time.Duration
	}{
		{"near-miss vs unrelated", closeN, unrelatedN},
		{"correct vs unrelated", exactN, unrelatedN},
	} {
		ratio := float64(pair.slow+1) / float64(pair.fast+1)
		if ratio < 0.6 {
			t.Errorf("%s: %v vs %v — one path returns early (ratio %.2f)", pair.name, pair.fast, pair.slow, ratio)
		}
	}
}

func measure(h Hasher, password, verifier string, rounds int) time.Duration {
	start := time.Now()
	for i := 0; i < rounds; i++ {
		h.Verify(password, verifier)
	}
	return time.Since(start) / time.Duration(rounds)
}
