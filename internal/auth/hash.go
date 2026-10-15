// Package auth is the single administrator account: how a password becomes a
// verifier, how a verifier becomes a session, and how fast somebody may find out
// they were wrong.
//
// It is deliberately small and deliberately stdlib-only. The admin signs in over
// the local network (docs/PLAN.md §6), which means this code is the only thing
// between a random host on the LAN and the switch that unlocks every child in the
// house. Everything here therefore fails closed: an unparseable verifier is not a
// login failure to retry, it is a broken credential that must be reported, and a
// cost parameter read out of stored data is bounded before it is obeyed.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// The stored encoding is `pbkdf2-sha256$<iterations>$<b64salt>$<b64hash>`.
//
// The scheme is written into the string rather than assumed so that a change of
// KDF is a migration rather than a mass lockout, and the iteration count is
// written per record rather than read from the code so the production cost can be
// raised without invalidating a credential somebody set months ago.
const scheme = "pbkdf2-sha256"

const (
	// DefaultIterations is the production cost. 600 000 is OWASP's current
	// recommendation for PBKDF2-HMAC-SHA256: about 250 ms on one core of a
	// 2020-class CPU, which is slow enough to make an offline crack expensive and
	// fast enough that a parent signing in once a day will not notice.
	DefaultIterations = 600_000

	// MinIterations is a floor on what a *stored* credential may claim.
	//
	// Without it, a config file edited to say "10" would still verify the correct
	// password — so one careless hand edit, or a single corrupted character, would
	// quietly replace the KDF with something a laptop cracks in seconds, and nothing
	// anywhere would say the credential had got weaker.
	//
	// It is enforced where stored data enters the system (PasswordAuthenticator),
	// not inside Parse. Parse is a decoder and has to decode what it is given; the
	// floor is a policy about trust, and the thing that decides whether a verifier
	// is trusted data or a test fixture is the caller that loaded it. Enforcing it
	// in Parse would also make the cheap test cost unusable, which is how this was
	// first written and why the first version of this suite could not verify
	// anything at all.
	MinIterations = 100_000

	// TestIterations is the cost for tests. Anything that exercises hashing
	// hundreds of times needs a cost three orders of magnitude below production,
	// and having one named value means the difference between "test cost" and
	// "someone typed a weak cost into /etc/plock/config.json" is visible.
	TestIterations = 1_000

	// MaxIterations bounds what the verify path will spend. The iteration count
	// comes out of stored data, and PBKDF2's whole purpose is to make work; an
	// unbounded read of it turns a config value into a way to make the login
	// handler burn hours per attempt, which is a denial of service on the only
	// interface that turns enforcement back on.
	//
	// Eight times the default leaves room to raise the cost for real without
	// hitting the ceiling.
	MaxIterations = 8_000_000

	saltBytes = 16
	keyBytes  = 32
)

// Errors returned by Parse and Verify. They are distinct because the caller has
// to do different things with them: ErrInvalidVerifier means the stored
// credential is broken and an operator has to be told, while a false from Verify
// means somebody typed the wrong password and the right response is to let them
// try again, slowly.
var (
	// ErrInvalidVerifier reports a stored credential this package cannot read.
	ErrInvalidVerifier = errors.New("malformed password verifier")

	// ErrWeakCost reports a verifier whose iteration count is below the floor.
	ErrWeakCost = fmt.Errorf("%w: iteration count below the accepted minimum", ErrInvalidVerifier)

	// ErrEmptyPassword reports an attempt to create a credential with no password.
	ErrEmptyPassword = errors.New("password is empty")
)

// Hasher derives and checks password verifiers.
//
// The cost is a field rather than a constant inside the functions so a test that
// hashes in a loop is not waiting on real KDF work. The zero value is the
// production cost, which means forgetting to set it makes things slower, never
// weaker.
type Hasher struct {
	// Iterations is the PBKDF2 cost used for Hash. Zero means DefaultIterations.
	// It does not affect Verify: that uses the cost recorded in the verifier,
	// because a hash made at another cost has to keep verifying or raising the
	// default would lock the parent out of their own machine.
	Iterations int
}

// randRead is the entropy source, as a variable so the failure path is reachable
// by a test. A salt fallback would be the bug here — a predictable salt is what
// makes two installs with the same password one dictionary attack — and a guard
// that has never been observed to fire is a guard nobody has checked.
var randRead = func(b []byte) error { _, err := rand.Read(b); return err }

// Params is a parsed verifier.
type Params struct {
	Iterations int
	Salt       []byte
	Key        []byte
}

// Hash turns a plaintext password into a stored verifier.
//
// The salt comes from crypto/rand and the error it returns is fatal to the
// operation rather than worked around: a salt from a fallback source is a salt an
// attacker can predict, and the one thing a salt buys is that two installs with
// the same password store different bytes.
func (h Hasher) Hash(password string) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}
	iter := h.Iterations
	if iter == 0 {
		iter = DefaultIterations
	}
	if err := checkCost(iter); err != nil {
		return "", err
	}

	salt := make([]byte, saltBytes)
	if err := randRead(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iter, keyBytes)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}
	return fmt.Sprintf("%s$%d$%s$%s", scheme, iter,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify reports whether password matches the stored verifier.
//
// It returns a bare bool and no error, deliberately: the caller is a login
// handler, and every distinction it might act on — wrong password versus
// unreadable credential versus unsupported scheme — has to be reported to the
// operator rather than to the person guessing, so the handler wants one bit for
// the response and the detail for the log. Use Parse directly when you have the
// detail.
//
// A verifier this package cannot read never verifies, whatever the password. That
// is the fail-closed direction: a corrupt credential means nobody gets in until
// an operator fixes it, which is an outage the parent can see and repair, rather
// than a login that succeeds against something nobody can characterise.
func (h Hasher) Verify(password, verifier string) bool {
	p, err := Parse(verifier)
	if err != nil {
		return false
	}
	return h.verifyWith(p, password)
}

// verifyWith is Verify with the verifier already parsed.
//
// The authenticator has to parse the stored string anyway, to check its cost before
// it spends a quarter-second deriving a key. Going through Verify would parse the same
// bytes a second time, and — the part that matters — the cost the KDF actually uses
// would come from the second parse rather than the one that was checked. That is inert
// today because both parses read the same string, and it stops being inert the first
// time the two paths differ. One parse, one cost, one answer.
func (h Hasher) verifyWith(p Params, password string) bool {
	if password == "" {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, password, p.Salt, p.Iterations, len(p.Key))
	if err != nil {
		return false
	}
	// Constant time, and comparing the derived key rather than an encoding of it:
	// the comparison has to take the same time whether the guess was one character
	// off or completely unrelated, or the KDF's work is wasted and an attacker
	// recovers the password a byte at a time.
	return subtle.ConstantTimeCompare(key, p.Key) == 1
}

// Parse reads a stored verifier.
//
// Every field is checked rather than coerced. base64 in particular is decoded
// strictly: a trailing newline, an alphabet swap, or padding added by a
// copy-paste through a text editor all produce a different salt, and a wrong salt
// with a right-shaped key is a credential that verifies nothing and looks fine.
func Parse(verifier string) (Params, error) {
	parts := strings.Split(verifier, "$")
	if len(parts) != 4 {
		return Params{}, fmt.Errorf("%w: expected 4 fields, got %d", ErrInvalidVerifier, len(parts))
	}
	if parts[0] != scheme {
		return Params{}, fmt.Errorf("%w: unsupported scheme %q", ErrInvalidVerifier, parts[0])
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil {
		return Params{}, fmt.Errorf("%w: iteration count %q is not a number", ErrInvalidVerifier, parts[1])
	}
	if err := checkCost(iter); err != nil {
		return Params{}, err
	}
	salt, err := decodeRaw(parts[2])
	if err != nil {
		return Params{}, fmt.Errorf("%w: salt: %v", ErrInvalidVerifier, err)
	}
	if len(salt) != saltBytes {
		return Params{}, fmt.Errorf("%w: salt is %d bytes, want %d", ErrInvalidVerifier, len(salt), saltBytes)
	}
	key, err := decodeRaw(parts[3])
	if err != nil {
		return Params{}, fmt.Errorf("%w: key: %v", ErrInvalidVerifier, err)
	}
	if len(key) != keyBytes {
		return Params{}, fmt.Errorf("%w: key is %d bytes, want %d", ErrInvalidVerifier, len(key), keyBytes)
	}
	return Params{Iterations: iter, Salt: salt, Key: key}, nil
}

// checkCost bounds the iteration count against what the process will spend.
//
// Only the ceiling is here. The floor is a policy about stored credentials and is
// enforced by CheckStoredCost; see MinIterations.
func checkCost(iter int) error {
	if iter < 1 {
		return fmt.Errorf("%w: %d iterations", ErrInvalidVerifier, iter)
	}
	if iter > MaxIterations {
		return fmt.Errorf("%w: %d iterations, maximum %d", ErrInvalidVerifier, iter, MaxIterations)
	}
	return nil
}

// CheckStoredCost reports whether a parsed verifier is strong enough to be the
// credential on a real machine.
//
// Call it on anything read from disk. It is separate from Parse so that a decoder
// stays a decoder, and so that the cheap cost a test suite runs on does not look
// like a policy violation — but a verifier loaded from /etc/plock and handed to a
// login handler without this check is a credential whose strength nobody verified.
func CheckStoredCost(p Params) error {
	if p.Iterations < MinIterations {
		return fmt.Errorf("%w: %d iterations, minimum %d", ErrWeakCost, p.Iterations, MinIterations)
	}
	return nil
}

// decodeRaw decodes unpadded standard base64 with no tolerance at all.
//
// StdEncoding with padding would accept a string this package never produced, and
// the two encodings differ in trailing characters only — so a lenient decode
// turns "the same credential, pasted differently" into two different keys.
func decodeRaw(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty field")
	}
	b, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	// Re-encode and require the same bytes. RawStdEncoding is canonical for its
	// input length, so a mismatch means somebody edited the tail of the field, and
	// the trailing bits they changed are bits of the salt or the key.
	if base64.RawStdEncoding.EncodeToString(b) != s {
		return nil, errors.New("not canonical base64")
	}
	return b, nil
}
