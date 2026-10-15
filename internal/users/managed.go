package users

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// noteCaseCollisions marks rows whose names differ only by case.
//
// They stay separate rows, because on Linux they really are separate accounts and
// joining them would silently hand one child another child's profile. What must not
// happen is the silent split: the common cause is a parent who wrote `Casey` in the
// config for the `casey` in /etc/passwd, and the result of that typo is a ghost account
// that cannot be locked out plus an unmanaged child -- one kid, two rows, and nothing on
// the page saying the two names are the same name. config's own validation only rejects
// empty and byte-identical duplicates, so nothing upstream prevents it.
//
// This records the judgement rather than resolving it, which is the rule this package
// already follows for every other ambiguity it cannot settle.
func noteCaseCollisions(byName map[string]*Managed, order []string) {
	byFold := make(map[string][]string, len(order))
	for _, name := range order {
		k := strings.ToLower(name)
		byFold[k] = append(byFold[k], name)
	}
	for _, names := range byFold {
		if len(names) < 2 {
			continue
		}
		sort.Strings(names)
		for i, name := range names {
			others := make([]string, 0, len(names)-1)
			for j, other := range names {
				if i != j {
					others = append(others, strconv.Quote(other))
				}
			}
			byName[name].Problems = append(byName[name].Problems,
				fmt.Sprintf("this name differs only by case from %s; they are treated as separate accounts",
					strings.Join(others, ", ")))
		}
	}
}

// Managed is one row of the account screen: what the machine says, what the config
// says, and the disagreement between them.
//
// The disagreement is the reason this type exists. A screen built from the password
// file alone cannot show an account that was configured and then removed from the
// machine — and that row is the one a parent needs, because an unmanaged ghost
// entry keeps its profile assignment and its override while becoming impossible to
// reach. A screen built from the config alone cannot show the child who has never
// been added. So both sources are kept, and the row says which ones it came from.
type Managed struct {
	Username string

	// OnMachine is the discovered account, or nil when the config names somebody the
	// password file no longer has.
	OnMachine *Account

	// Configured is the config's entry, or nil for an account that exists but has
	// never been managed. Its ProfileID and Enabled are the config's answers, not
	// this package's opinion.
	Configured *ConfiguredUser

	// Stale marks the row that is in the config but not on the machine.
	//
	// It is a field rather than something the template derives from OnMachine == nil
	// because the template should not be deciding what an absence means: a row that
	// cannot be locked out any more is a different thing to show than a row that can,
	// and that judgement belongs next to the code that made it.
	Stale bool

	// Problems are the disagreements this package noticed but could not settle. They
	// are rendered on the row rather than only logged, because the person who can fix
	// a name typed into the wrong box is the person looking at this page.
	Problems []string
}

// ConfiguredUser is the slice of a config entry this package needs.
//
// It is a local type rather than config.User on purpose. internal/users must not
// import internal/config: config is the thing being edited by the screens this
// feeds, and an import the other way would make the account listing part of the
// write path's dependency graph. One three-field struct is a cheaper price than
// that cycle.
type ConfiguredUser struct {
	Username  string
	ProfileID string
	Enabled   bool
}

// Union joins discovered accounts with configured ones.
//
// Ordering is by username and is total, because the screen paginates and a row that
// moves between loads is worse than a slow load.
//
// Both inputs are taken as slices rather than a config so the caller decides what
// "configured" means — the whole document, or one profile's members — without this
// package knowing anything about documents.
func Union(discovered []Account, configured []ConfiguredUser) []Managed {
	byName := make(map[string]*Managed, len(discovered)+len(configured))
	order := make([]string, 0, len(discovered)+len(configured))

	touch := func(name string) *Managed {
		if m, ok := byName[name]; ok {
			return m
		}
		m := &Managed{Username: name}
		byName[name] = m
		order = append(order, name)
		return m
	}

	// Both rows are copied rather than pointed into the caller's slices.
	//
	// internal/config's edit helpers already carry this rule for the other direction:
	// a document handed to two holders must never be written through an element of a
	// slice either holder might still own. Holding a pointer into configured is the
	// same hazard seen from the reader's side -- the config layer is copy-on-write, so
	// an edit produces a new slice and leaves this row reading a stale document that
	// nothing still calls current. A copy makes the row a snapshot, which is what a
	// rendered page actually is.
	for i := range discovered {
		a := discovered[i]
		touch(a.Username).OnMachine = &a
	}
	for i := range configured {
		c := configured[i]
		touch(c.Username).Configured = &c
	}
	for _, m := range byName {
		// The judgement is made here rather than in the template. See Managed.Stale.
		m.Stale = m.OnMachine == nil
	}
	noteCaseCollisions(byName, order)

	sort.Strings(order)
	out := make([]Managed, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out
}

// Unmanaged counts the rows that are not currently protecting anybody.
//
// The definition is the whole function, and it is here rather than at each call
// site so there is one answer to "are we covered". Three shapes qualify:
//
//   - never configured, the obvious case;
//   - configured but switched off, which a count of "has a config row" would
//     silently treat as done on exactly the machine where nothing has been done;
//   - configured and enabled but no longer on the machine.
//
// The third is the one worth arguing for, because the code says the parent did
// everything right. Enabled means the daemon will try to enforce it, and the account
// it would enforce is gone, so the row protects nobody while the dashboard reports
// it as managed. A number that counts that as coverage is the most dangerous kind of
// number: it is true about the config and false about the children.
func Unmanaged(rows []Managed) int {
	n := 0
	for _, r := range rows {
		if r.Stale || r.Configured == nil || !r.Configured.Enabled {
			n++
		}
	}
	return n
}
