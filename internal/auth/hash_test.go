package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
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

// The timing bound the ticket asks for -- with its claim limited to what a wall clock can
// actually resolve here.
//
// Measured: pbkdf2.Key at the production floor is 4.3 ms and the 32-byte
// ConstantTimeCompare is 53 ns, so the comparison is 0.0012% of a Verify. This test's
// floor is a ratio of 0.6, a 40% difference. It therefore guards the expensive half and
// nothing else: a "fast path" that skips or caches the KDF moves 4.3 ms and is visible,
// while a per-byte oracle in the comparison is not, and never was. That was verified
// rather than reasoned -- an injected early-return comparison passed this test three times
// running on both the mean and the minimum statistic -- and TestKeyComparisonIsConstantTime
// in constant_time_test.go is what now covers the comparison.
//
// What this test still catches on its own: comparing a length before the bytes, or any
// change that makes the work depend on the guess rather than on the iteration count.
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
	// One loop, three guesses, in a fixed order that is not the order they are
	// compared in. Measured one-after-the-other, the first guess pays for the pages
	// the later ones get for free, and a systematic gap appears between paths that do
	// identical work. Interleaving puts every guess in the same page-cache state.
	taken := measureTogether(h, verifier, rounds, close, unrelated, secret)
	closeN, unrelatedN, exactN := taken[0], taken[1], taken[2]

	// A correct-prefix wrong password must not be materially faster than a completely
	// wrong one, and neither must the correct password: an early return shows up as one
	// path finishing sooner, never later.
	//
	// That direction is the only one that means anything, and the previous version of
	// this assertion failed on the other one. It divided the two numbers whichever way
	// round they came out, so a near-miss measured as *slower* -- which is what a loaded
	// runner actually produces, because preemption only ever adds time -- tripped it.
	// CI caught it at ratio 0.58 with the near-miss at 145 ms against an unrelated guess
	// at 85 ms: a guess that takes longer than another guess is not a side channel, and
	// no early return can make a path slower. Taking the minimum did not fix that,
	// because the minimum of a path that is genuinely slower on this runner is still
	// slower; the bug was the assertion, not the statistic.
	for _, pair := range []struct {
		name            string
		candidate, base time.Duration
	}{
		{"near-miss vs unrelated", closeN, unrelatedN},
		{"correct vs unrelated", exactN, unrelatedN},
	} {
		if ratio, suspect := suspectEarlyReturn(pair.candidate, pair.base); suspect {
			// Not "one path returns early". That is one possible cause and the least
			// likely one; what this measurement can actually establish is that the two
			// paths were not indistinguishable on this run. Saying it that way keeps a
			// noisy runner from reporting a side channel in the password verifier as a
			// finding -- which is what the old message did, and it is the reason #44 was
			// opened.
			t.Errorf("%s: %v vs %v (ratio %.2f) -- the first is materially faster, which an early return is one explanation for; inconclusive on this run, not proof of one; PBKDF2's cost is fixed, so re-run before believing it",
				pair.name, pair.candidate, pair.base, ratio)
		}
	}
}

// suspectEarlyReturn reports whether a candidate path ran materially faster than the
// baseline it is being compared against, and with what ratio.
//
// Only the faster direction is a finding. An early return makes a path finish sooner;
// nothing in this package can make a wrong guess cost less work by being slower, so a
// candidate measured at 145 ms against an 85 ms baseline is a runner that preempted
// one loop, not a side channel. The assertion on main divides the two numbers whichever
// way round they come out and so fails on that noise -- which is exactly how #53's CI
// went red at ratio 0.58 on a branch that does not touch this package.
func suspectEarlyReturn(candidate, base time.Duration) (float64, bool) {
	if candidate >= base {
		return 1, false
	}
	ratio := float64(candidate+1) / float64(base+1)
	return ratio, ratio < 0.6
}

// measureTogether times several guesses against one verifier in an interleaved order
// and returns the shortest round each one had, in the order given.
//
// The interleaving is the point; see the note at the call site. The per-guess minimum
// keeps the reason measure() takes a minimum: preemption can only add time.
func measureTogether(h Hasher, verifier string, rounds int, passwords ...string) []time.Duration {
	best := make([]time.Duration, len(passwords))
	for i := range best {
		best[i] = time.Duration(math.MaxInt64)
	}
	for r := 0; r < rounds; r++ {
		for i, p := range passwords {
			start := time.Now()
			_ = h.Verify(p, verifier)
			if d := time.Since(start); d < best[i] {
				best[i] = d
			}
		}
	}
	return best
}

// The direction is the whole content of this assertion, so it gets asserted directly
// rather than only through a measurement that can only ever be suggestive. A runner
// that preempts one loop makes a same-cost path look slower; if that counts as a
// finding, the suite goes red on unrelated branches, which is what happened to #53.
func TestASlowerCandidateIsNotTreatedAsAnEarlyReturn(t *testing.T) {
	const base = 85 * time.Millisecond
	for _, tc := range []struct {
		name      string
		candidate time.Duration
		suspect   bool
	}{
		{"identical", base, false},
		// The CI measurement that turned a green branch red: the near-miss took
		// longer, which is not a side channel in any reading.
		{"near-miss slower, as CI measured it", 145 * time.Millisecond, false},
		{"slower still", time.Second, false},
		{"marginally faster is noise", 80 * time.Millisecond, false},
		// A path that skips most of the KDF is the shape this test exists for, and a
		// 16x gap is what injecting one actually measured.
		{"materially faster", base / 16, true},
		{"returns immediately", time.Microsecond, true},
	} {
		if _, got := suspectEarlyReturn(tc.candidate, base); got != tc.suspect {
			t.Errorf("%s: suspectEarlyReturn(%v, %v) = %v, want %v",
				tc.name, tc.candidate, base, got, tc.suspect)
		}
	}
}
