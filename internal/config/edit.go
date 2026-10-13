package config

import (
	"fmt"
	"time"
)

// The helpers on this file are what the web UI calls. They are permissive —
// they will happily create a state that Validate rejects — because a form
// submission wants to make several edits before checking the result. Call
// Validate before Save.
//
// None of them write into a slice the caller might still be holding. A `snap :=
// *cfg` shares the config's arrays, and a helper that filtered in place would
// leave that snapshot looking like a valid document about the wrong child.
//
// That covers two shapes, not one, and only the first is obvious:
//
//   - writing through an existing element, or filtering with `in[:0]`, which moves
//     data the snapshot can still see;
//   - appending. One append is harmless — the write lands past the snapshot's
//     length. But Load's slices arrive with spare capacity, because encoding/json
//     grows by doubling, so two configs sharing one array each appending collide in
//     that spare slot. Neither holder is writing through the other's elements, and
//     both end up with the wrong document.
//
// So every helper allocates: copySlice before an element write, and copySlice
// before an append. The second rule is the one that got missed on AddOverride, and
// the shape it produces fails open — an unlock appears for a child nobody granted
// one — so it is pinned by TestAddOverrideDoesNotWriteIntoAnArrayAnotherConfigHolds
// rather than left to the reader.

// UpsertProfile adds p, or replaces the profile that already has its ID.
//
// It writes into a fresh array rather than overwriting the existing element, so
// that a `snap := *c` taken before the call still describes what it described
// when it was taken. See the note at the top of this file.
func (c *Config) UpsertProfile(p Profile) {
	next := copySlice(c.Profiles)
	for i, existing := range next {
		if existing.ID == p.ID {
			next[i] = p
			c.Profiles = next
			return
		}
	}
	c.Profiles = append(next, p)
}

// DeleteProfile removes a profile by ID.
//
// It refuses while an account still points at that profile. Dropping it out
// from under a user would leave a dangling profileID, which Validate rejects
// and the enforcement loop would have to guess a meaning for — silently
// enforcing nothing is the wrong guess.
func (c *Config) DeleteProfile(id string) error {
	for _, u := range c.Users {
		if u.ProfileID == id {
			return fmt.Errorf("config: profile %q is still assigned to account %q; reassign that account first",
				id, u.Username)
		}
	}

	kept, found := cut(c.Profiles, func(p Profile) bool { return p.ID == id })
	if !found {
		return fmt.Errorf("config: no profile %q", id)
	}
	c.Profiles = kept
	return nil
}

// UpsertUser assigns a profile to an account, replacing any earlier assignment.
// Like UpsertProfile it writes into a fresh array, so a snapshot taken before the
// call is not quietly rewritten by it — which matters most here, because the
// field being replaced is the one that switches a child's lockout on or off.
func (c *Config) UpsertUser(u User) {
	next := copySlice(c.Users)
	for i, existing := range next {
		if existing.Username == u.Username {
			next[i] = u
			c.Users = next
			return
		}
	}
	c.Users = append(next, u)
}

// RemoveUser unassigns an account and drops any override it holds, so removing
// a child's profile does not leave a grant behind that the next person to use
// that account would inherit.
func (c *Config) RemoveUser(username string) error {
	kept, found := cut(c.Users, func(u User) bool { return u.Username == username })
	if !found {
		return fmt.Errorf("config: no account %q", username)
	}
	c.Users = kept
	c.RemoveOverride(username)
	return nil
}

// UserFor returns the assignment record for an account.
//
// It returns a copy. A pointer into c.Users describes whatever ends up at that
// index, which changes when a later RemoveUser shifts the list — and the web UI
// is exactly the caller that holds one of these while it edits.
func (c *Config) UserFor(username string) (User, bool) {
	for _, u := range c.Users {
		if u.Username == username {
			return u, true
		}
	}
	return User{}, false
}

// ProfileFor returns the profile an account is locked out by. It reports false
// for an account that is not configured, one whose enforcement is switched off,
// and one whose profile no longer exists — all three mean "nothing to enforce",
// and the caller has no reason to tell them apart.
//
// The Profile is a copy of the record; its rule and override lists are shared
// with the config, which is safe because nothing in this package edits a
// profile's slices in place — UpsertProfile replaces the whole value.
func (c *Config) ProfileFor(username string) (Profile, bool) {
	u, ok := c.UserFor(username)
	if !ok || !u.Enabled {
		return Profile{}, false
	}
	for _, p := range c.Profiles {
		if p.ID == u.ProfileID {
			return p, true
		}
	}
	return Profile{}, false
}

// AddOverride grants a temporary unlock, replacing whatever override that
// account already holds. Replacing rather than appending is what keeps the list
// bounded and the UI honest: there is only ever one answer to "may this account
// be unlocked right now".
//
// The deadline is reduced to UTC without a monotonic reading, so the value the
// caller keeps is the value that will come back from Load.
func (c *Config) AddOverride(o Override) {
	o.Until = instant(o.Until)
	c.RemoveOverride(o.Username)
	// copySlice, not an append into c.Overrides: two configs taken from one Load
	// share that array, and encoding/json leaves spare capacity in it, so a plain
	// append writes the new grant into the slot the other config is about to use.
	// See TestAddOverrideDoesNotWriteIntoAnArrayAnotherConfigHolds.
	c.Overrides = append(copySlice(c.Overrides), o)
}

// RemoveOverride withdraws an account's override, reporting whether there was
// one to withdraw.
func (c *Config) RemoveOverride(username string) bool {
	kept, found := cut(c.Overrides, func(o Override) bool { return o.Username == username })
	if found {
		c.Overrides = kept
	}
	return found
}

// ActiveOverride returns the override currently granting an account access, if
// any. An override whose Until has passed is not active, and the enforcement
// loop relies on that being true the instant the deadline goes by.
func (c *Config) ActiveOverride(username string, now time.Time) (Override, bool) {
	for _, o := range c.Overrides {
		if o.Username != username {
			continue
		}
		if o.Until.After(now) {
			return o, true
		}
	}
	return Override{}, false
}

// cut returns a new slice holding the elements pred rejects, and whether it
// dropped anything.
//
// It allocates instead of filtering into the receiver's own array. The in-place
// idiom is the fast one, and the one that bites here: `snap := *cfg` is a
// supported way to keep hold of the document a form was submitted from, and an
// in-place filter leaves that snapshot its original length with a shifted tail —
// a document that still reads as valid and names the wrong child. A
// configuration is a few kilobytes and these run on a parent's click, not in the
// enforcement loop, so the allocation is not the thing worth optimising.
func cut[T any](in []T, pred func(T) bool) ([]T, bool) {
	kept := make([]T, 0, len(in))
	found := false

	for _, v := range in {
		if pred(v) {
			found = true
			continue
		}
		kept = append(kept, v)
	}
	return kept, found
}
