package users

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// readLoginDefs pulls the uid range out of login.defs.
//
// The file is a shell-ish list of KEY value lines with whole-line comments, and it
// is written by a distribution maintainer, so it is read leniently and reported
// verbosely rather than parsed strictly. A malformed line must not blank the admin
// screen: the cost of being strict is a dashboard that cannot list anyone, which is
// the failure this package exists to avoid.
//
// Absent keys fall back to shadow-utils' own defaults, and every fallback is
// announced in the notes. The defaults are not neutral: DefaultUIDMin of 1000 hides
// the first human on an old Red Hat system, where humans start at 500.
func readLoginDefs(path string) (lo, hi uint32, notes []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, nil, err
	}
	defer f.Close()

	// "the key is in the file" and "the key gave us a usable number" are different
	// facts; the notes have to know which one failed.
	minSeen, maxSeen := false, false
	lo, hi = DefaultUIDMin, DefaultUIDMax

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for lineNo := 1; sc.Scan(); lineNo++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Fields, not Cut: the separator between a key and its value is "any run of
		// whitespace", which is what a maintainer actually types and what a copy
		// from a man page arrives as. Anything past the second token is not part of
		// the value, so a trailing comment does not poison the number.
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToUpper(parts[0])
		value := parts[1]

		switch key {
		case "UID_MIN":
			// Seen and usable are two facts, and conflating them made the file lie
			// about itself: a UID_MIN that was present but unreadable produced both
			// "is not a number" and "sets no UID_MIN", and the second sentence sent an
			// operator to grep for a key sitting in front of them.
			minSeen = true
			v, err := parseDefsUint(value)
			if err != nil {
				notes = append(notes, fmt.Sprintf("%s line %d: UID_MIN %q %s; using %d", path, lineNo, value, whyNotABound(err), DefaultUIDMin))
				break
			}
			lo = v
		case "UID_MAX":
			maxSeen = true
			v, err := parseDefsUint(value)
			if err != nil {
				notes = append(notes, fmt.Sprintf("%s line %d: UID_MAX %q %s; using %d", path, lineNo, value, whyNotABound(err), DefaultUIDMax))
				break
			}
			hi = v
		}
	}
	if err := sc.Err(); err != nil {
		return 0, 0, nil, err
	}

	if !minSeen {
		notes = append(notes, fmt.Sprintf("%s sets no UID_MIN; assuming %d, which hides any account below it", path, lo))
	}
	if !maxSeen {
		notes = append(notes, fmt.Sprintf("%s sets no UID_MAX; assuming %d", path, hi))
	}
	// An inverted range is not a range a human meant. It is reported rather than
	// silently swapped: swapping invents a policy, and the operator needs to see that
	// the file contradicts itself.
	if lo > hi {
		notes = append(notes, fmt.Sprintf("%s: UID_MIN %d is above UID_MAX %d, so no account can be in range", path, lo, hi))
	}
	return lo, hi, notes, nil
}

// parseUint accepts a plain non-negative decimal and nothing else. That is the grammar
// passwd(5) gives a uid and a gid, and it is the grammar used for /etc/passwd only --
// login.defs numbers are read by parseDefsUint, which is wider.
//
// It is a thin wrapper rather than a hand-rolled filter, because base 10 already gives
// exactly that grammar: ParseUint with an explicit base rejects a sign, an exponent, and
// a 0x prefix (measured: "+1000", "-1000", "1e3" and "0x10" are all "invalid syntax"). An
// earlier version of this function also checked for a leading "+" by hand. That check was
// dead — no input reaches ParseUint that it would have caught and ParseUint would have
// accepted — and dead code that reads like a policy is worse than none, because the next
// reader looks to that line for the rule instead of to the call that actually enforces it.
//
// The behaviour is still pinned by TestAPlusPrefixedBoundIsNotAcceptedSilently. That
// test guards the contract against a future edit that swaps in ParseInt or base 0,
// which is the realistic way this stops being true.
func parseUint(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

// parseDefsUint reads a login.defs numeric parameter in the grammar login.defs(5)
// actually documents: "Numbers (both regular and long) may be either decimal values,
// octal values (precede the value with 0) or hexadecimal values (precede the value with
// 0x)".
//
// Base 10 is narrower than the file format, and the divergence goes the dangerous way
// without saying so. `UID_MIN 01750` is octal for 1000; read as decimal it is 1750, which
// quietly raises the floor and removes every child between 1000 and 1749 from the only
// screen where a parent could add a rule for them -- with no note, because the value
// parsed "fine". Base 0 is the documented grammar exactly, and it is narrower than it
// sounds: ParseUint still rejects a sign, whitespace and an exponent, so "+1000",
// "-1000" and "1e3" stay errors.
//
// The two things base 0 accepts that login.defs does not document are Go's underscore
// separator and a 0b binary prefix, so both are refused by hand rather than let through.
// A maintainer who writes `UID_MIN 0b100` has made a mistake, and the mistake this
// package must never make is quietly agreeing to one.
func parseDefsUint(s string) (uint32, error) {
	if strings.ContainsRune(s, '_') {
		return 0, fmt.Errorf("%q: underscores are not part of the login.defs number format", s)
	}
	if len(s) > 2 && (s[0] == '0' && (s[1] == 'b' || s[1] == 'B')) {
		return 0, fmt.Errorf("%q: binary is not part of the login.defs number format", s)
	}
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

// whyNotABound words the two ways a login.defs number can fail, which are different
// problems to have. "is not a number" sent an operator to squint at a value that is
// perfectly well formed and simply does not fit the field.
func whyNotABound(err error) string {
	if errors.Is(err, strconv.ErrRange) {
		return "is a number too large for this file to express"
	}
	return "is not a number in any of the bases login.defs allows"
}

// noLoginShells are the shells that mean "this account cannot hold a session".
//
// The list is by exact path rather than by substring. Matching "nologin" loosely
// would also match a real shell installed under a name containing it, and the
// direction of that error is hiding a child.
//
// An empty shell field is deliberately NOT in this list. passwd(5) says that when the
// shell field is empty it "defaults to the value /bin/sh", so a record with nothing
// written in field 7 is an account that can log in and can be locked out. An earlier
// version treated it as a no-login shell on the reasoning that shadow refuses such an
// account a session; that is the opposite of what the format specifies, and it dropped
// the account with no note, which is the one failure mode this package exists to
// prevent.
var noLoginShells = map[string]bool{
	"/bin/false":        true,
	"/usr/bin/false":    true,
	"/sbin/false":       true,
	"/bin/nologin":      true,
	"/usr/bin/nologin":  true,
	"/usr/sbin/nologin": true,
	"/sbin/nologin":     true,
}

// defaultShell is what passwd(5) specifies for a record whose shell field is empty.
// It is named rather than inlined because the users page has to say the same thing the
// parser decided, and two copies of a spec'd default are two chances to disagree.
const defaultShell = "/bin/sh"

// declaredNoHome are home values that are a statement rather than a path.
//
// This is where the package reads the requirement "exclude accounts with no home"
// narrowly, and the narrowness is deliberate. A home directory that is merely
// absent from disk right now — an unmounted volume, an NFS home that has not
// mounted yet, a profile created before the home was copied — is not a declaration
// that the account has no home, and dropping such an account removes a real child
// from the only screen where a parent could add a rule for them. So a missing
// directory is kept and noted, and only these sentinels, which are what
// useradd(8) writes when it is told there is no home, exclude anyone.
var declaredNoHome = map[string]bool{
	"":             true,
	"/nonexistent": true,
	"/nowhere":     true,
}

// parsePasswd applies the filter, recording every judgement it had to make.
//
// The notes are not decoration. Every dropped record is a way for a child to go
// unlisted, and a parent staring at a short list needs to be told which records were
// dropped and why, in the same breath as the list itself.
func parsePasswd(r io.Reader, lo, hi uint32, homeExists func(string) bool, out *Listing) ([]Account, error) {
	var (
		accounts []Account
		seen     = map[string]bool{}
	)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for lineNo := 1; sc.Scan(); lineNo++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		// Seven is the documented shape. A short record cannot be interpreted at
		// all, so it is skipped — but it is skipped out loud, because a truncated
		// password file is exactly the event that makes accounts vanish.
		if len(fields) < 7 {
			out.Notes = append(out.Notes, fmt.Sprintf("line %d has %d fields, not 7; skipped", lineNo, len(fields)))
			continue
		}
		if len(fields) > 7 {
			// Extra trailing fields are common enough (some NSS exports carry a
			// comment column) and harmless: the first seven are the record.
			out.Notes = append(out.Notes, fmt.Sprintf("line %d has %d fields; the extra ones were ignored", lineNo, len(fields)))
		}

		name, home, shell := fields[0], fields[5], fields[6]
		uid, err := parseUint(fields[2])
		if err != nil {
			out.Notes = append(out.Notes, fmt.Sprintf("line %d (%s): uid %q is not a number; skipped", lineNo, name, fields[2]))
			continue
		}
		gid, err := parseUint(fields[3])
		if err != nil {
			out.Notes = append(out.Notes, fmt.Sprintf("line %d (%s): gid %q is not a number; skipped", lineNo, name, fields[3]))
			continue
		}

		if name == "" {
			out.Notes = append(out.Notes, fmt.Sprintf("line %d has an empty login name; skipped", lineNo))
			continue
		}
		if seen[name] {
			out.Notes = append(out.Notes, fmt.Sprintf("%s appears more than once; the first entry was used", name))
			continue
		}

		if uid < lo || uid > hi {
			// Not a note: this is the filter working, and it fires for hundreds of
			// system accounts on a normal machine.
			continue
		}
		if shell == "" {
			// Kept, and noted. passwd(5) defaults an empty shell field to /bin/sh, so
			// this is a person with a shell, and the daemon just inferred the shell
			// rather than reading one. That inference is exactly the kind of judgement
			// this package records.
			out.Notes = append(out.Notes, fmt.Sprintf("%s (uid %d) declares no login shell; %s is used", name, uid, defaultShell))
		} else if noLoginShells[shell] {
			// Silent, and for the same reason as the uid filter: a machine has dozens of
			// nologin service accounts and a note per one would bury the real notes.
			continue
		}
		if declaredNoHome[home] {
			out.Notes = append(out.Notes, fmt.Sprintf("%s (uid %d) declares no home directory; excluded", name, uid))
			continue
		}
		if !homeExists(home) {
			// Kept. See declaredNoHome for why this is the safe direction.
			out.Notes = append(out.Notes, fmt.Sprintf("%s (uid %d) has home %s, which does not exist; kept anyway", name, uid, home))
		}

		seen[name] = true
		accounts = append(accounts, Account{Username: name, UID: uid, GID: gid, Home: home, Shell: shell})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}
