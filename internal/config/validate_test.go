package config

import (
	"strings"
	"testing"
	"time"
)

// valid is the configuration every rejection test starts from and breaks in one
// place. UTC keeps the suite independent of the machine's zone database.
func valid() *Config {
	c := sample()
	c.Profiles[0].Timezone = "UTC"
	c.Overrides = nil
	return c
}

func TestValidateAcceptsAGoodConfig(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("Validate on a good config: %v", err)
	}
	if err := Default().Validate(); err == nil {
		t.Error("Validate accepted a config with no administrator")
	}
}

func TestValidateRejectsEachMalformedField(t *testing.T) {
	cases := []struct {
		name   string
		mangle func(*Config)
		want   string
	}{
		{
			name:   "wrong schema version",
			mangle: func(c *Config) { c.Version = 999 },
			want:   "version",
		},
		{
			name:   "no administrator",
			mangle: func(c *Config) { c.Admin.Username = "" },
			want:   "admin.username",
		},
		{
			name:   "blank administrator",
			mangle: func(c *Config) { c.Admin.Username = "   " },
			want:   "admin.username",
		},
		{
			// Every login would fail with nothing on disk to check against.
			name:   "no password verifier",
			mangle: func(c *Config) { c.Admin.Password = "" },
			want:   "admin.password",
		},
		{
			name:   "empty listen address",
			mangle: func(c *Config) { c.Settings.Listen = "" },
			want:   "settings.listen",
		},
		{
			name:   "poll interval below the floor",
			mangle: func(c *Config) { c.Settings.PollSeconds = 4 },
			want:   "settings.pollSeconds",
		},
		{
			name:   "zero poll interval",
			mangle: func(c *Config) { c.Settings.PollSeconds = 0 },
			want:   "settings.pollSeconds",
		},
		{
			name:   "poll interval above the ceiling",
			mangle: func(c *Config) { c.Settings.PollSeconds = MaxPollSeconds + 1 },
			want:   "settings.pollSeconds",
		},
		{
			// The consumer multiplies this into a time.Duration; unbounded, it
			// wraps to a negative interval.
			name:   "poll interval that overflows a Duration",
			mangle: func(c *Config) { c.Settings.PollSeconds = 9223372036854775807 },
			want:   "settings.pollSeconds",
		},
		{
			name:   "unknown tls mode",
			mangle: func(c *Config) { c.Settings.TLS = "yes please" },
			want:   "settings.tls",
		},
		{
			name: "custom tls without paths",
			mangle: func(c *Config) {
				c.Settings.TLS = "custom"
				c.Settings.CertFile = ""
				c.Settings.KeyFile = ""
			},
			want: "settings.certFile",
		},
		{
			name:   "no lock backends",
			mangle: func(c *Config) { c.Settings.LockBackends = nil },
			want:   "settings.lockBackends",
		},
		{
			name:   "empty profile id",
			mangle: func(c *Config) { c.Profiles[0].ID = "" },
			want:   "profiles[0] (School night).id",
		},
		{
			name: "duplicate profile id",
			mangle: func(c *Config) {
				c.Profiles = append(c.Profiles, Profile{ID: "schoolnight", Timezone: "UTC"})
			},
			want: `id "schoolnight" is used more than once`,
		},
		{
			name:   "bad timezone",
			mangle: func(c *Config) { c.Profiles[0].Timezone = "Mars/Olympus_Mons" },
			want:   "timezone",
		},
		{
			name:   "empty timezone",
			mangle: func(c *Config) { c.Profiles[0].Timezone = "" },
			want:   "timezone",
		},
		{
			name:   "empty rule id",
			mangle: func(c *Config) { c.Profiles[0].Rules[0].ID = "" },
			want:   "rules[0] (Bedtime).id",
		},
		{
			name: "duplicate rule id",
			mangle: func(c *Config) {
				c.Profiles[0].Rules[1].ID = c.Profiles[0].Rules[0].ID
			},
			want: `rule id "r1" is used more than once`,
		},
		{
			name:   "start without a leading zero",
			mangle: func(c *Config) { c.Profiles[0].Rules[0].Start = "9:00" },
			want:   "rules[0] (Bedtime).start",
		},
		{
			name:   "start out of range",
			mangle: func(c *Config) { c.Profiles[0].Rules[0].Start = "24:00" },
			want:   "start",
		},
		{
			name:   "end out of range",
			mangle: func(c *Config) { c.Profiles[0].Rules[1].End = "13:60" },
			want:   "end",
		},
		{
			name:   "end with seconds",
			mangle: func(c *Config) { c.Profiles[0].Rules[1].End = "13:30:00" },
			want:   "end",
		},
		{
			name:   "no days",
			mangle: func(c *Config) { c.Profiles[0].Rules[0].Days = nil },
			want:   "days",
		},
		{
			name:   "unknown day name",
			mangle: func(c *Config) { c.Profiles[0].Rules[0].Days = []string{"mon", "fun"} },
			want:   `"fun" is not a weekday name`,
		},
		{
			name:   "capitalised day name",
			mangle: func(c *Config) { c.Profiles[0].Rules[0].Days = []string{"Mon"} },
			want:   "is not a weekday name",
		},
		{
			name:   "day listed twice",
			mangle: func(c *Config) { c.Profiles[0].Rules[0].Days = []string{"mon", "mon"} },
			want:   "listed more than once",
		},
		{
			name:   "user points at a missing profile",
			mangle: func(c *Config) { c.Users[0].ProfileID = "nope" },
			want:   "users[0].profileID",
		},
		{
			name:   "empty username",
			mangle: func(c *Config) { c.Users[0].Username = "" },
			want:   "users[0].username",
		},
		{
			name: "duplicate account",
			mangle: func(c *Config) {
				c.Users = append(c.Users, User{Username: "kiddo", ProfileID: "schoolnight"})
			},
			want: `account "kiddo" is listed more than once`,
		},
		{
			name: "override for an unknown account",
			mangle: func(c *Config) {
				c.Overrides = []Override{{Username: "ghost", Until: time.Now().Add(time.Hour)}}
			},
			want: "overrides[0].username",
		},
		{
			name: "override without a deadline",
			mangle: func(c *Config) {
				c.Overrides = []Override{{Username: "kiddo"}}
			},
			want: "overrides[0].until",
		},
		{
			// AddOverride keeps one per account; a hand edit can hold several, and
			// then which one wins is array order.
			name: "two overrides for one account",
			mangle: func(c *Config) {
				until := time.Now().Add(time.Hour)
				c.Overrides = []Override{
					{Username: "kiddo", Until: until, Reason: "first"},
					{Username: "kiddo", Until: until.Add(time.Hour), Reason: "second"},
				}
			},
			want: "more than one override",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mangle(c)

			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate accepted a config with %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to name %q", err, tc.want)
			}
		})
	}
}

// A form that reports one problem per submission is a form the parent gives up
// on, so Validate has to keep going after the first failure.
func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	c := valid()
	c.Version = 7
	c.Admin.Username = ""
	c.Settings.PollSeconds = 1
	c.Profiles[0].Rules[0].Start = "nope"

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate accepted four broken fields")
	}
	for _, want := range []string{"version", "admin.username", "settings.pollSeconds", "start"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %v does not mention %q", err, want)
		}
	}
	if n := strings.Count(err.Error(), "; ") + 1; n < 4 {
		t.Errorf("error lists %d problems, want at least 4: %v", n, err)
	}
}

func TestValidateAcceptsEveryTlsMode(t *testing.T) {
	for _, mode := range tlsModes {
		c := valid()
		c.Settings.TLS = mode
		if mode == "custom" {
			c.Settings.CertFile = "/var/lib/plock/tls/cert.pem"
			c.Settings.KeyFile = "/var/lib/plock/tls/key.pem"
		}
		if err := c.Validate(); err != nil {
			t.Errorf("tls %q: %v", mode, err)
		}
	}
}

// A window that crosses midnight is the documented shape of "21:00 to 06:30",
// so it must not be mistaken for a malformed rule. 00:00–23:59 is a plain
// window of 1439 minutes, which is as close to all day as two HH:MM boxes get.
func TestValidateAcceptsOvernightWindows(t *testing.T) {
	c := valid()
	c.Profiles[0].Rules = []Rule{
		{ID: "overnight", Days: []string{"sun"}, Start: "21:00", End: "06:30", Enabled: true},
		{ID: "almost-all-day", Days: []string{"sat"}, Start: "00:00", End: "23:59", Enabled: true},
	}
	if err := c.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// Start == End is read as overnight, so it covers all 1440 minutes. That is the
// opposite of what a parent who typed the same time into both boxes means, and
// it is the one typo in this file that silently locks an account permanently.
func TestValidateRejectsAWindowThatEndsWhereItStarts(t *testing.T) {
	c := valid()
	c.Profiles[0].Rules = []Rule{
		{ID: "typo", Days: []string{"mon"}, Start: "08:00", End: "08:00", Enabled: true},
	}

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate accepted a window that starts and ends at the same time")
	}
	if !strings.Contains(err.Error(), "rules[0].start") {
		t.Errorf("problem is not attributed to rules[0].start: %v", err)
	}
	if !strings.Contains(err.Error(), "all day") {
		t.Errorf("problem does not say what the window actually does: %v", err)
	}
}

func TestIsDay(t *testing.T) {
	for _, d := range DayNames {
		if !IsDay(d) {
			t.Errorf("IsDay(%q) = false, want true", d)
		}
	}
	for _, d := range []string{"", "Mon", "monday", "sun ", "saturn"} {
		if IsDay(d) {
			t.Errorf("IsDay(%q) = true, want false", d)
		}
	}
}
