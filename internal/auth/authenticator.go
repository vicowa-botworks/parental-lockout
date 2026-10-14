package auth

import (
	"crypto/subtle"
	"errors"
)

// Authenticator decides whether a username and password are the administrator's.
//
// It is here as the documented place a different mechanism goes, not as an
// abstraction several implementations share. The one that matters is PAM: signing
// in with the real system account is what an enterprise deployment would want,
// and it is deliberately not implemented, because it needs cgo and a module and
// this project has a hard zero-dependency rule. Leaving the seam named and empty
// means that decision is recorded where somebody will find it rather than being
// re-litigated in a pull request.
//
// An implementation must not return true alongside a non-nil error. The caller
// treats an error as "no answer" and reports it to the operator; a true would be
// read as a login by whoever reads the code next.
type Authenticator interface {
	Authenticate(username, password string) (bool, error)
}

// PasswordAuthenticator is the implementation this package actually uses: one
// stored verifier, checked with PBKDF2.
type PasswordAuthenticator struct {
	// Hasher carries the cost. It only affects anything that hashes; Authenticate
	// uses the cost recorded in the stored verifier.
	Hasher Hasher
	// Username and Verifier are the stored credential, as held in the config.
	Username string
	Verifier string
}

// ErrNoCredential reports an admin record with no verifier in it.
//
// This is a distinct error rather than a plain "wrong password" because the two
// look identical from outside and are nothing alike to fix: one is somebody
// guessing, and the other is a machine where nobody can ever sign in.
var ErrNoCredential = errors.New("no administrator credential is configured")

// Authenticate checks the password against the stored verifier.
//
// A missing verifier is an error, and an unreadable verifier is an error, and
// neither is a false: the handler has to be able to tell "you got it wrong" from
// "this machine cannot authenticate anybody", because only the second one means
// calling somebody.
func (a PasswordAuthenticator) Authenticate(username, password string) (bool, error) {
	if a.Verifier == "" {
		return false, ErrNoCredential
	}
	p, err := Parse(a.Verifier)
	if err != nil {
		return false, err
	}
	// The stored credential is checked for strength here, at the point where a
	// string from /etc/plock becomes an answer to a login attempt, rather than
	// inside the decoder. This is the only place that knows the bytes came from
	// disk, and it is the only place the distinction between "a cheap test fixture"
	// and "a credential somebody weakened" can be made.
	if err := CheckStoredCost(p); err != nil {
		return false, err
	}
	// The username is compared after the KDF, not before. Comparing it first would
	// answer in microseconds for a wrong username and a quarter of a second for the
	// right one, which hands the attacker a free oracle for the half of the
	// credential they do not know — and the username is not secret, so the only
	// reason not to leak it is that leaking it is how a guess becomes targeted.
	ok := a.Hasher.Verify(password, a.Verifier)
	if !usernameMatches(username, a.Username) {
		return false, nil
	}
	return ok, nil
}

// usernameMatches compares usernames case-insensitively.
//
// Case-insensitive because a parent types "Parent" at 11pm and means the account
// they created as "parent".
//
// It is not, and does not claim to be, hidden in time: ConstantTimeCompare returns
// immediately when the lengths differ, and folding is a loop over the input. That
// is fine because the username is not the secret — the whole design assumes an
// attacker may know it. What had to be hidden was whether the *password* was
// right, which is why the KDF above runs before this and why its result is
// discarded rather than short-circuited.
func usernameMatches(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(asciiFold(got)), []byte(asciiFold(want))) == 1
}

// asciiFold lowercases ASCII letters and leaves everything else alone.
//
// strings.EqualFold is not used because it folds Unicode, and a username stored as
// one codepoint and typed as another that folds to it would be a login this
// package grants on a comparison the config store does not make. Restricting the
// fold to ASCII keeps "these are the same account" meaning the same thing here as
// it does everywhere the username is written down.
func asciiFold(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + ('a' - 'A')
		}
	}
	return string(out)
}
