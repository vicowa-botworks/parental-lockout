package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// timeOfDay is the documented wall-clock format. It is deliberately strict:
// "9:00" would parse happily with a more forgiving pattern and then never match,
// because comparison is done on the zero-padded string.
var timeOfDay = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// tlsModes are the settings the web server knows how to act on: generate a
// self-signed certificate, serve plain HTTP, or use operator-supplied paths.
var tlsModes = []string{"auto", "off", "custom"}

// Validate reports every problem it finds, not just the first. The web UI shows
// this list to a parent who has just submitted a form, and making them fix one
// error per round trip is a poor way to spend an evening.
//
// Each message names the field it is about, because the UI has nothing else to
// point at.
//
// Save does not call this. The daemon has to keep enforcing whatever is already
// on disk even if a hand edit made it unsound, so refusing to write is the
// responsibility of whoever produced the change.
func (c *Config) Validate() error {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if c.Version != CurrentVersion {
		add("version: got %d, this build writes %d", c.Version, CurrentVersion)
	}

	if strings.TrimSpace(c.Admin.Username) == "" {
		add("admin.username: an administrator account is required")
	}

	if strings.TrimSpace(c.Settings.Listen) == "" {
		add("settings.listen: must not be empty")
	}
	if c.Settings.PollSeconds < 5 {
		add("settings.pollSeconds: %d is below the minimum of 5", c.Settings.PollSeconds)
	}
	if !contains(tlsModes, c.Settings.TLS) {
		add("settings.tls: %q is not one of %s", c.Settings.TLS, strings.Join(tlsModes, ", "))
	}
	if c.Settings.TLS == "custom" && (c.Settings.CertFile == "" || c.Settings.KeyFile == "") {
		add("settings.certFile and settings.keyFile: both are required when tls is \"custom\"")
	}
	if len(c.Settings.LockBackends) == 0 {
		add("settings.lockBackends: at least one lock backend is required")
	}

	profileIDs := make(map[string]bool, len(c.Profiles))
	for i, p := range c.Profiles {
		label := fmt.Sprintf("profiles[%d]", i)
		if p.Name != "" {
			label = fmt.Sprintf("profiles[%d] (%s)", i, p.Name)
		}

		if strings.TrimSpace(p.ID) == "" {
			add("%s.id: must not be empty", label)
		} else if profileIDs[p.ID] {
			add("profiles: id %q is used more than once", p.ID)
		} else {
			profileIDs[p.ID] = true
		}

		// LoadLocation("") answers UTC rather than failing, so an unset
		// timezone would silently evaluate every rule in the machine's zone.
		if p.Timezone == "" {
			add("%s.timezone: must not be empty", label)
		} else if _, err := time.LoadLocation(p.Timezone); err != nil {
			add("%s.timezone: %q is not a usable timezone: %v", label, p.Timezone, err)
		}

		ruleIDs := make(map[string]bool, len(p.Rules))
		for j, r := range p.Rules {
			rl := fmt.Sprintf("%s.rules[%d]", label, j)
			if r.Name != "" {
				rl = fmt.Sprintf("%s.rules[%d] (%s)", label, j, r.Name)
			}

			if strings.TrimSpace(r.ID) == "" {
				add("%s.id: must not be empty", rl)
			} else if ruleIDs[r.ID] {
				add("%s: rule id %q is used more than once in %s", rl, r.ID, label)
			} else {
				ruleIDs[r.ID] = true
			}

			if !timeOfDay.MatchString(r.Start) {
				add("%s.start: %q is not HH:MM on a 24-hour clock", rl, r.Start)
			}
			if !timeOfDay.MatchString(r.End) {
				add("%s.end: %q is not HH:MM on a 24-hour clock", rl, r.End)
			}

			if len(r.Days) == 0 {
				add("%s.days: a rule with no days can never match", rl)
			}
			seen := make(map[string]bool, len(r.Days))
			for _, d := range r.Days {
				switch {
				case !IsDay(d):
					add("%s.days: %q is not a weekday name (%s)", rl, d, strings.Join(DayNames, ", "))
				case seen[d]:
					add("%s.days: %q is listed more than once", rl, d)
				default:
					seen[d] = true
				}
			}
		}
	}

	usernames := make(map[string]bool, len(c.Users))
	for i, u := range c.Users {
		label := fmt.Sprintf("users[%d]", i)
		if strings.TrimSpace(u.Username) == "" {
			add("%s.username: must not be empty", label)
		} else if usernames[u.Username] {
			add("users: account %q is listed more than once", u.Username)
		} else {
			usernames[u.Username] = true
		}

		if !profileIDs[u.ProfileID] {
			add("%s.profileID: %q does not name an existing profile", label, u.ProfileID)
		}
	}

	for i, o := range c.Overrides {
		label := fmt.Sprintf("overrides[%d]", i)
		if !usernames[o.Username] {
			add("%s.username: %q does not name a configured account, so this override would do nothing",
				label, o.Username)
		}
		if o.Until.IsZero() {
			add("%s.until: must be a date and time", label)
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("config: %d problem(s): %s", len(problems), strings.Join(problems, "; "))
	}
	return nil
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
