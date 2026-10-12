package config

import (
	"fmt"
	"time"
)

// The helpers on this file are what the web UI calls. They are permissive —
// they will happily create a state that Validate rejects — because a form
// submission wants to make several edits before checking the result. Call
// Validate before Save.

// UpsertProfile adds p, or replaces the profile that already has its ID.
func (c *Config) UpsertProfile(p Profile) {
	for i, existing := range c.Profiles {
		if existing.ID == p.ID {
			c.Profiles[i] = p
			return
		}
	}
	c.Profiles = append(c.Profiles, p)
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
	for i, p := range c.Profiles {
		if p.ID == id {
			c.Profiles = append(c.Profiles[:i], c.Profiles[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("config: no profile %q", id)
}

// UpsertUser assigns a profile to an account, replacing any earlier assignment.
func (c *Config) UpsertUser(u User) {
	for i, existing := range c.Users {
		if existing.Username == u.Username {
			c.Users[i] = u
			return
		}
	}
	c.Users = append(c.Users, u)
}

// RemoveUser unassigns an account and drops any override it holds, so removing
// a child's profile does not leave a grant behind that the next person to use
// that account would inherit.
func (c *Config) RemoveUser(username string) error {
	kept := c.Users[:0]
	found := false
	for _, u := range c.Users {
		if u.Username == username {
			found = true
			continue
		}
		kept = append(kept, u)
	}
	if !found {
		return fmt.Errorf("config: no account %q", username)
	}
	c.Users = kept
	c.RemoveOverride(username)
	return nil
}

// UserFor returns the assignment record for an account.
func (c *Config) UserFor(username string) (*User, bool) {
	for i := range c.Users {
		if c.Users[i].Username == username {
			return &c.Users[i], true
		}
	}
	return nil, false
}

// ProfileFor returns the profile an account is locked out by. It reports false
// for an account that is not configured, one whose enforcement is switched off,
// and one whose profile no longer exists — all three mean "nothing to enforce",
// and the caller has no reason to tell them apart.
func (c *Config) ProfileFor(username string) (*Profile, bool) {
	u, ok := c.UserFor(username)
	if !ok || !u.Enabled {
		return nil, false
	}
	for i := range c.Profiles {
		if c.Profiles[i].ID == u.ProfileID {
			return &c.Profiles[i], true
		}
	}
	return nil, false
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
	c.Overrides = append(c.Overrides, o)
}

// RemoveOverride withdraws an account's override, reporting whether there was
// one to withdraw.
func (c *Config) RemoveOverride(username string) bool {
	kept := c.Overrides[:0]
	removed := false
	for _, o := range c.Overrides {
		if o.Username == username {
			removed = true
			continue
		}
		kept = append(kept, o)
	}
	c.Overrides = kept
	return removed
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
