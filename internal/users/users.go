// Package users answers one question for the admin UI: which accounts on this
// machine look like people rather than services.
//
// It is a listing, not an enforcement decision. Nothing here can lock anybody out,
// and that asymmetry sets every default in the package. A missed account never
// crashes anything and never logs a warning: the parent opens the dashboard, sees
// no child, concludes there is nothing to configure, and the machine enforces
// nothing while looking healthy. That is the same silence this project treats as
// the dangerous failure everywhere else, so when a record is odd in a way that
// could go either way the account is kept and the oddity is reported. Showing a
// printer-service account the parent can ignore costs one glance; hiding a real
// account costs the whole feature.
package users

import (
	"errors"
	"fmt"
	"os"
)

// Account is one human-looking entry in the password file.
type Account struct {
	// Username is the login name, exactly as the password file spells it. Nothing
	// here uses it to build a path or a command, but callers that do should treat it
	// as attacker-influenced text: the file is root-owned, yet a machine may carry
	// accounts created by somebody else entirely.
	Username string
	UID      uint32
	GID      uint32
	Home     string
	// Shell is field 7 verbatim, including the values that mean "no login".
	//
	// Kept as the raw string rather than a boolean so a screen can show why an
	// account was kept or dropped instead of asserting a conclusion it cannot
	// support. Two different shells mean "no interactive login" and they are worth
	// telling apart when the question on screen is "why isn't my kid listed?".
	Shell string
}

// Source names the files to read. The zero value is the real machine.
//
// Both paths exist because every rule in this package is a rule about text a
// distribution maintainer wrote, and a test cannot make /etc/login.defs say
// something else without the seam. The alternative — asserting against the host's
// files — produces a suite that passes here and means nothing on the next machine.
type Source struct {
	PasswdPath    string
	LoginDefsPath string

	// HomeExists reports whether a home directory is present. nil means os.Stat.
	//
	// A field rather than a call for the same reason the platform package takes a
	// Runner: whether /home/whoever exists is a fact about the host, and the
	// interesting cases — a home that was never created, a home spelled
	// /nonexistent — cannot be conjured on a test machine on demand.
	HomeExists func(path string) bool
}

// Default paths, exported so a caller can build on them rather than repeat the
// literals.
const (
	DefaultPasswdPath    = "/etc/passwd"
	DefaultLoginDefsPath = "/etc/login.defs"
)

// DefaultUIDMin is what shadow-utils itself assumes when login.defs is silent.
//
// It is a floor, and a floor that is too high is the dangerous direction here: an
// old Red Hat system puts its first human at 500, and assuming 1000 would hide that
// child from the only screen where a parent could add them. That is why List
// reports the assumption in Notes rather than swallowing it.
const DefaultUIDMin = 1000

// DefaultUIDMax likewise mirrors shadow-utils. An account above the ceiling is
// normally a system or container account, and the ceiling is reported the same way.
const DefaultUIDMax = 60000

// ErrNoAccounts means the password file was read successfully and nothing in it
// looked human.
//
// A distinct error rather than an empty result because the two mean opposite things
// to an operator. An empty list is a fact about the machine; this is a fact about
// the configuration, and the usual cause is a UID_MIN read as something absurd — a
// value of 60000 on a system that allocates from 1000 upward excludes everyone.
var ErrNoAccounts = errors.New("users: no human-looking accounts in the password file")

// Listing is the answer plus the reasoning behind it.
type Listing struct {
	Accounts []Account

	// UIDMin and UIDMax are the bounds that were actually applied, whether they came
	// from login.defs or from the defaults. A parent debugging "where is my kid"
	// needs the numbers that were used, not the ones a file was supposed to contain.
	UIDMin uint32
	UIDMax uint32

	// Notes explains every place the package had to guess, and every record it could
	// not read cleanly. Empty means the files said what they meant.
	//
	// This exists because the alternative is a page that quietly disagrees with the
	// operator's mental model, and a silently wrong admin screen is worse than one
	// that admits it is guessing.
	Notes []string
}

// List reads the account list together with the UID bounds that filter it.
//
// They are read together on purpose. The filter is meaningless without the bounds,
// and a caller that fetched them separately could pair a fresh password file with a
// stale floor — the shape where an edit to login.defs silently changes who counts
// as human between two reads of the same page.
func List(src Source) (*Listing, error) {
	passwd, defs, homes := src.resolve()

	lo, hi, notes, err := readLoginDefs(defs)
	if err != nil {
		return nil, fmt.Errorf("users: reading the uid range from %s: %w", defs, err)
	}

	f, err := os.Open(passwd)
	if err != nil {
		return nil, fmt.Errorf("users: opening %s: %w", passwd, err)
	}
	defer f.Close()

	out := &Listing{UIDMin: lo, UIDMax: hi, Notes: notes}
	accounts, err := parsePasswd(f, lo, hi, homes, out)
	if err != nil {
		return nil, fmt.Errorf("users: reading %s: %w", passwd, err)
	}
	if len(accounts) == 0 {
		// The bounds travel with the failure. "no accounts" with no explanation
		// sends an operator to grep a file they already looked at; the numbers that
		// were applied are the thing they need.
		return &Listing{UIDMin: lo, UIDMax: hi, Notes: out.Notes}, ErrNoAccounts
	}
	out.Accounts = accounts
	return out, nil
}

// resolve fills in whatever the caller left unset. It is a method rather than
// inline so that "the zero Source means the real machine" is a fact a test can read
// off the code instead of one it has to discover by opening files.
func (s Source) resolve() (passwd, defs string, homes func(string) bool) {
	passwd, defs = s.PasswdPath, s.LoginDefsPath
	if passwd == "" {
		passwd = DefaultPasswdPath
	}
	if defs == "" {
		defs = DefaultLoginDefsPath
	}
	homes = s.HomeExists
	if homes == nil {
		homes = pathExists
	}
	return passwd, defs, homes
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
