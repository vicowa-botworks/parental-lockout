package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Every switch in this document decides whether a child gets locked out, so
// every one of them has to default to enforcing. These are the cases where a
// plausible-looking document would otherwise enforce nothing, with no error
// anywhere to say so.
func TestOmittedSwitchesDefaultToEnforcing(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		check func(t *testing.T, c *Config)
	}{
		{
			// The one that matters most: a user written without "enabled" must
			// still be locked out. ProfileFor returning false here means the
			// enforcement loop skips the account forever.
			name: "user without enabled",
			body: `{"version":1,"admin":{"username":"parent"},
				"profiles":[{"id":"p","timezone":"UTC","rules":[{"id":"r","days":["mon"],"start":"21:00","end":"06:00"}]}],
				"users":[{"username":"kiddo","profileID":"p"}]}`,
			check: func(t *testing.T, c *Config) {
				u, ok := c.UserFor("kiddo")
				if !ok || !u.Enabled {
					t.Errorf("UserFor(kiddo) = %+v, %v; want an enabled account", u, ok)
				}
				if _, ok := c.ProfileFor("kiddo"); !ok {
					t.Error("ProfileFor found nothing to enforce for an account that was never disabled")
				}
			},
		},
		{
			name: "rule without enabled",
			body: `{"version":1,"admin":{"username":"parent"},
				"profiles":[{"id":"p","timezone":"UTC","rules":[{"id":"r","days":["mon"],"start":"21:00","end":"06:00"}]}]}`,
			check: func(t *testing.T, c *Config) {
				if !c.Profiles[0].Rules[0].Enabled {
					t.Error("a rule that never mentioned enabled loaded as disabled")
				}
			},
		},
		{
			name:  "settings without enforcement",
			body:  `{"version":1,"settings":{"pollSeconds":60}}`,
			check: func(t *testing.T, c *Config) { _ = c },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			write(t, path, tc.body)

			c, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !c.Settings.Enforcement {
				t.Error("settings.enforcement = false, want the fail-closed default of true")
			}
			tc.check(t, c)
		})
	}
}

// An explicit false still means false — the point is to default an omission,
// not to overrule a decision.
func TestExplicitFalseIsHonoured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	write(t, path, `{"version":1,"settings":{"enforcement":false},
		"profiles":[{"id":"p","timezone":"UTC","rules":[{"id":"r","days":["mon"],"start":"21:00","end":"06:00","enabled":false}]}],
		"users":[{"username":"kiddo","profileID":"p","enabled":false}]}`)

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Settings.Enforcement {
		t.Error("settings.enforcement = true, want the explicit false")
	}
	if c.Profiles[0].Rules[0].Enabled {
		t.Error("rule enabled = true, want the explicit false")
	}
	if u, _ := c.UserFor("kiddo"); u.Enabled {
		t.Error("user enabled = true, want the explicit false")
	}
}

// encoding/json keeps the last of two identical keys and says nothing. In a
// document that decides whether a machine is locked, a value that disappears
// without an error has to be refused rather than resolved by precedence.
func TestLoadRejectsDuplicateKeys(t *testing.T) {
	cases := map[string]string{
		"twice at the top level": `{"version":1,"version":1}`,
		// The hard case: the value before the repeated key is an object, so a
		// walker that forgets the container was a value reads the key as data.
		"an object written twice":      `{"version":1,"admin":{"username":"a"},"admin":{"username":"b"}}`,
		"an array value then a repeat": `{"version":1,"profiles":[],"profiles":[]}`,
		// The fail-open direction, which is the reason this is checked at all.
		"enforcement written twice": `{"version":1,"settings":{"enforcement":true,"enforcement":false}}`,
		"user enabled twice":        `{"version":1,"users":[{"username":"kiddo","profileID":"p","enabled":true,"enabled":false}]}`,
		"rule enabled twice":        `{"version":1,"profiles":[{"id":"p","timezone":"UTC","rules":[{"id":"r","days":["mon"],"start":"21:00","end":"06:00","enabled":true,"enabled":false}]}]}`,
		// Same key in two different objects is not a duplicate.
		"nested objects sharing a key name": `{"version":1,"admin":{"username":"a"},"users":[{"username":"b","profileID":"p"}]}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			write(t, path, body)

			_, err := Load(path)
			duplicates := !strings.Contains(name, "sharing a key name")
			if duplicates && err == nil {
				t.Fatalf("Load accepted a document with a repeated key: %s", body)
			}
			if duplicates && !strings.Contains(err.Error(), "duplicate key") {
				t.Errorf("error = %v, want it to say duplicate key", err)
			}
			if !duplicates && err != nil {
				t.Errorf("Load rejected the same key in two different objects: %v", err)
			}
		})
	}
}

// Criterion 1 is that Save then Load is the identity. A time.Time from
// time.Now() carries a monotonic reading and one built in a named zone carries
// a *Location, and neither survives RFC 3339 — so without normalization the
// identity fails for exactly the values AddOverride is called with in
// production.
func TestRoundTripIdentityAcrossTimeShapes(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("no timezone database: %v", err)
	}
	ist := time.FixedZone("IST", 5*3600+1800)

	cases := map[string]time.Time{
		"monotonic clock": time.Now(),
		"named zone":      time.Date(2026, 10, 12, 20, 40, 0, 0, berlin),
		"fixed offset":    time.Date(2026, 10, 12, 20, 40, 0, 0, ist),
		"utc":             time.Date(2026, 10, 12, 20, 40, 0, 0, time.UTC),
		"zero":            {},
	}

	for name, when := range cases {
		t.Run("override "+name, func(t *testing.T) {
			c := valid()
			c.AddOverride(Override{Username: "kiddo", Until: when.Add(time.Hour), Reason: "r", By: "parent"})
			roundTripIsIdentity(t, c)
		})
		t.Run("admin "+name, func(t *testing.T) {
			c := valid()
			c.Admin.UpdatedAt = when
			roundTripIsIdentity(t, c)
		})
	}
}

func roundTripIsIdentity(t *testing.T, in *Config) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")

	if err := in.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := in.normalized(); !reflect.DeepEqual(back, want) {
		t.Errorf("Load(Save(c)) != c.normalized():\n got %+v\nwant %+v", back, want)
	}

	// And the document itself must be stable under a second save.
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := back.Save(path); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("the document changed on a second save:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
