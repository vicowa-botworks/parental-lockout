// Package config owns the single JSON document that is the whole persistent
// state of the application: who the administrator is, which lockout profiles
// exist, which child account is assigned to which profile, and any temporary
// overrides in force.
//
// Everything else in the program reads and writes that document through this
// package, so the on-disk rules — atomic replacement, mode 0600, refusal of a
// version it does not understand — are stated once here rather than being
// re-derived at every call site.
//
// # Fail closed
//
// This program decides when a child may use a computer. Every ambiguity in the
// document therefore resolves toward enforcing: a missing "enforcement" or
// "enabled" means on, not off. Reading an omitted switch as false is the one
// failure mode that would let a child use a machine their parent believes is
// locked, and it is invisible — no error, no log, just a rule that quietly does
// nothing.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// CurrentVersion is the schema version this program reads and writes. Load
// refuses any other value: silently reading a newer document with fields
// dropped would let the enforcement loop act on a schedule the administrator
// did not write.
const CurrentVersion = 1

// Default values applied on load, so a hand-edited document that omits a
// setting still produces a working configuration. Enforcement and the backend
// list have no named default: they are the zero value plus one line in
// UnmarshalJSON, and naming them here would only invite the two to drift.
const (
	DefaultListen      = "0.0.0.0:8443"
	DefaultTLS         = "auto"
	DefaultPollSeconds = 30
)

// Config is the whole document.
type Config struct {
	Version   int        `json:"version"`
	Admin     Admin      `json:"admin"`
	Settings  Settings   `json:"settings"`
	Profiles  []Profile  `json:"profiles"`
	Users     []User     `json:"users"`
	Overrides []Override `json:"overrides"`
}

// configJSON mirrors Config with Settings as a pointer, so that a document with
// no "settings" object at all can be given the full set of defaults instead of
// the zero value. Without this, `{"version":1}` would decode to a configuration
// with enforcement switched off.
type configJSON struct {
	Version   int        `json:"version"`
	Admin     Admin      `json:"admin"`
	Settings  *Settings  `json:"settings"`
	Profiles  []Profile  `json:"profiles"`
	Users     []User     `json:"users"`
	Overrides []Override `json:"overrides"`
}

// UnmarshalJSON decodes the document, keeping the difference between an absent
// section and an empty one.
//
// Defining it steps outside the outer decoder's DisallowUnknownFields setting,
// so the strictness is re-established here: a hand-edited document is a
// supported way to change this file, and a misspelled key that decodes to
// nothing is how a parent ends up believing they changed a setting they did not.
func (c *Config) UnmarshalJSON(data []byte) error {
	if dup := firstDuplicateKey(data); dup != "" {
		return fmt.Errorf("duplicate key %q; a key written twice silently keeps the last value", dup)
	}

	var raw configJSON
	if err := strictDecode(data, &raw); err != nil {
		return err
	}

	*c = Config{
		Version:   raw.Version,
		Admin:     raw.Admin,
		Profiles:  raw.Profiles,
		Users:     raw.Users,
		Overrides: raw.Overrides,
	}
	if raw.Settings != nil {
		c.Settings = *raw.Settings
	} else {
		c.Settings = defaultSettings()
	}
	return nil
}

// strictDecode decodes one JSON object into v with unknown keys rejected.
//
// Every custom unmarshaler in this package has to call it: defining
// UnmarshalJSON takes a type out of the outer decoder's hands, so its
// DisallowUnknownFields setting — and with it the typo guard for the whole
// document — would otherwise be lost for exactly the sections a parent edits by
// hand.
func strictDecode(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// firstDuplicateKey reports a key that appears twice in the same JSON object, or
// "" when every key is unique.
//
// encoding/json keeps the last occurrence and says nothing, so
// `{"enforcement":true,"enforcement":false}` means false with no error
// anywhere. A value that vanishes from a document deciding whether a child is
// locked out has to be an error, not a precedence rule.
func firstDuplicateKey(data []byte) string {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	type frame struct {
		isObject  bool
		expectKey bool
		keys      map[string]bool
	}
	var stack []frame

	for {
		tok, err := dec.Token()
		if err != nil {
			// A syntax error is the decoder's to report, not ours.
			return ""
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{', '[':
				// The container is itself a value when we were not looking for
				// a key, and the parent has to know it has been given one.
				// Without this, `{"a":{"b":1},"a":{"b":2}}` reads the second
				// "a" as a value and the duplicate goes unnoticed.
				if len(stack) > 0 {
					if parent := &stack[len(stack)-1]; parent.isObject && !parent.expectKey {
						parent.expectKey = true
					}
				}
				if delim == '{' {
					stack = append(stack, frame{isObject: true, expectKey: true, keys: make(map[string]bool)})
				} else {
					stack = append(stack, frame{})
				}
			default:
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
			continue
		}
		if len(stack) == 0 {
			return ""
		}
		top := &stack[len(stack)-1]
		if !top.isObject {
			continue
		}
		if !top.expectKey {
			top.expectKey = true
			continue
		}
		key, _ := tok.(string)
		if top.keys[key] {
			return key
		}
		top.keys[key] = true
		top.expectKey = false
	}
}

// Admin is the single administrator account the web UI signs in to. The
// password is a PBKDF2-SHA256 verifier produced by ticket #10, never plaintext.
type Admin struct {
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Settings are the daemon's own options.
type Settings struct {
	Listen       string   `json:"listen"`
	TLS          string   `json:"tls"`
	CertFile     string   `json:"certFile,omitempty"`
	KeyFile      string   `json:"keyFile,omitempty"`
	PollSeconds  int      `json:"pollSeconds"`
	Enforcement  bool     `json:"enforcement"`
	LockBackends []string `json:"lockBackends"`
}

// settingsJSON exists for one reason: to tell an omitted "enforcement" apart
// from an explicit false. A plain bool decodes both to false, and the wrong
// reading of a hand-edited document would silently switch the entire
// application off.
type settingsJSON struct {
	Listen       string   `json:"listen"`
	TLS          string   `json:"tls"`
	CertFile     string   `json:"certFile,omitempty"`
	KeyFile      string   `json:"keyFile,omitempty"`
	PollSeconds  int      `json:"pollSeconds"`
	Enforcement  *bool    `json:"enforcement"`
	LockBackends []string `json:"lockBackends"`
}

// UnmarshalJSON applies the enforcement default while the information that it
// was omitted is still available.
func (s *Settings) UnmarshalJSON(data []byte) error {
	var raw settingsJSON
	if err := strictDecode(data, &raw); err != nil {
		return err
	}

	*s = Settings{
		Listen:       raw.Listen,
		TLS:          raw.TLS,
		CertFile:     raw.CertFile,
		KeyFile:      raw.KeyFile,
		PollSeconds:  raw.PollSeconds,
		LockBackends: raw.LockBackends,
	}
	s.Enforcement = raw.Enforcement == nil || *raw.Enforcement
	return nil
}

// Profile is a named set of lockout windows with its own timezone.
type Profile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	Rules    []Rule `json:"rules"`
}

// Rule is a wall-clock window on a set of weekdays, both ends HH:MM in the
// profile's timezone.
//
// End earlier than Start means the window crosses midnight and belongs to the
// starting day: Sunday 21:00-06:30 runs into Monday morning.
//
// End equal to Start enforces nothing, and Validate rejects it. It is not a
// 24-hour window: reading it that way would turn a parent typing the same time
// into both boxes into a child who can never use the machine.
//
// Note the consequence while #16 is open: because a window is half-open and both
// ends are whole minutes, no rule covers every minute of a day — 00:00-23:59
// leaves the last minute free. A schedule that must never release is not
// expressible yet, and #16 decides whether it becomes a flag or a reserved value.
type Rule struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Days    []string `json:"days"`
	Start   string   `json:"start"`
	End     string   `json:"end"`
	Enabled bool     `json:"enabled"`
}

// ruleJSON exists so that a rule written without "enabled" is an enabled rule.
// A rule is created to lock a child out; defaulting it to disabled would make
// the omission the one way to switch a rule off without saying so.
type ruleJSON struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Days    []string `json:"days"`
	Start   string   `json:"start"`
	End     string   `json:"end"`
	Enabled *bool    `json:"enabled"`
}

// UnmarshalJSON defaults Enabled to true for a rule that does not mention it.
func (r *Rule) UnmarshalJSON(data []byte) error {
	var raw ruleJSON
	if err := strictDecode(data, &raw); err != nil {
		return err
	}

	*r = Rule{ID: raw.ID, Name: raw.Name, Days: raw.Days, Start: raw.Start, End: raw.End}
	r.Enabled = raw.Enabled == nil || *raw.Enabled
	return nil
}

// User assigns exactly one profile to one local account.
type User struct {
	Username  string `json:"username"`
	ProfileID string `json:"profileID"`
	Enabled   bool   `json:"enabled"`
}

// userJSON exists for the same reason as ruleJSON: assigning an account to a
// profile is an request to enforce it.
type userJSON struct {
	Username  string `json:"username"`
	ProfileID string `json:"profileID"`
	Enabled   *bool  `json:"enabled"`
}

// UnmarshalJSON defaults Enabled to true for an account that does not mention
// it.
func (u *User) UnmarshalJSON(data []byte) error {
	var raw userJSON
	if err := strictDecode(data, &raw); err != nil {
		return err
	}

	*u = User{Username: raw.Username, ProfileID: raw.ProfileID}
	u.Enabled = raw.Enabled == nil || *raw.Enabled
	return nil
}

// Override is a temporary unlock with an expiry, so a parent can grant "30 more
// minutes" without editing the schedule.
type Override struct {
	Username string    `json:"username"`
	Until    time.Time `json:"until"`
	Reason   string    `json:"reason"`
	By       string    `json:"by"`
}

// Default returns a configuration that is ready to enforce: no profiles, no
// users and no administrator yet, but the documented settings.
func Default() *Config {
	c := &Config{Version: CurrentVersion, Settings: defaultSettings()}
	c.applyDefaults()
	return c
}

// defaultSettings is what a document with no "settings" object at all means.
func defaultSettings() Settings {
	s := Settings{Enforcement: true}
	s.applyDefaults()
	return s
}

// applyDefaults fills in the settings a hand-edited document may omit.
//
// Enforcement is deliberately absent from this list. By the time it runs, an
// omitted key and an explicit false are the same value; that distinction is
// made in UnmarshalJSON, where the difference is still visible.
func (s *Settings) applyDefaults() {
	if s.Listen == "" {
		s.Listen = DefaultListen
	}
	if s.TLS == "" {
		s.TLS = DefaultTLS
	}
	if s.PollSeconds == 0 {
		s.PollSeconds = DefaultPollSeconds
	}
	if s.LockBackends == nil {
		s.LockBackends = []string{"logind", "kde", "gnome"}
	}
}

// applyDefaults fills in what a decoded or hand-built document may leave out,
// and puts every timestamp in the one form the document stores.
//
// It is called by Load after decoding and by Save before encoding, which is
// what makes Save-then-Load an identity: the bytes on disk are always the
// normalized form, and normalizing them again changes nothing.
func (c *Config) applyDefaults() {
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	c.Settings.applyDefaults()

	c.Admin.UpdatedAt = instant(c.Admin.UpdatedAt)
	for i := range c.Overrides {
		c.Overrides[i].Until = instant(c.Overrides[i].Until)
	}
}

// clipAll removes the spare capacity from every slice a decoded document carries.
//
// encoding/json grows slices by doubling, so a slice that leaves a decoder
// routinely has len < cap. Two holders of one document that each append to it then
// write into the same spare slot, and each ends up holding the other's document —
// the shape internal/config/edit.go's header exists to prevent, and the second
// shape that header had to grow to describe.
//
// Clipping at the source makes that shape impossible instead of relying on every
// helper to remember: with len == cap, any append reallocates on its own and cannot
// reach a sibling. This is the structural half of the defence, deliberately chosen
// over "the comment now lists both shapes" — a rule that has already failed twice is
// not a load-bearing barrier.
//
// The edit helpers stay copy-on-write regardless. Removing the hazard is not the
// same as retiring the rule: a Config built by hand, or by a future constructor that
// does not go through Load, can still carry room to append into.
func (c *Config) clipAll() {
	c.Profiles = slices.Clip(c.Profiles)
	c.Users = slices.Clip(c.Users)
	c.Overrides = slices.Clip(c.Overrides)
	c.Settings.LockBackends = slices.Clip(c.Settings.LockBackends)
	// The nested lists are the easy ones to forget, and they are exactly as
	// appendable as the top-level three.
	for i := range c.Profiles {
		c.Profiles[i].Rules = slices.Clip(c.Profiles[i].Rules)
		for j := range c.Profiles[i].Rules {
			c.Profiles[i].Rules[j].Days = slices.Clip(c.Profiles[i].Rules[j].Days)
		}
	}
}

// instant reduces a timestamp to what JSON can round-trip: UTC, with the
// monotonic reading dropped.
//
// A time.Time from time.Now() carries a monotonic clock, and one built with a
// named zone carries a *Location pointer; neither survives a trip through
// RFC 3339, so without this the value that comes back from Load is never
// DeepEqual to the value that went in, even though the bytes match.
func instant(t time.Time) time.Time {
	return t.UTC().Round(0)
}

// normalized returns a copy with defaults applied, leaving the receiver alone:
// Save must not silently mutate the caller's value, and the web UI keeps the
// submitted document around to show the parent what they typed.
//
// The copy is deep. A shallow one would alias Profiles, Rules and Days, so an
// edit made through the copy — which is exactly what Save is about to do while
// normalizing — would be visible in the caller's document.
func (c *Config) normalized() *Config {
	out := *c
	out.Admin = c.Admin
	out.Settings = c.Settings
	out.Settings.LockBackends = copySlice(c.Settings.LockBackends)

	if c.Profiles != nil {
		out.Profiles = make([]Profile, len(c.Profiles))
		for i, p := range c.Profiles {
			out.Profiles[i] = p
			if p.Rules != nil {
				out.Profiles[i].Rules = make([]Rule, len(p.Rules))
				copy(out.Profiles[i].Rules, p.Rules)
				for j := range out.Profiles[i].Rules {
					out.Profiles[i].Rules[j].Days = copySlice(p.Rules[j].Days)
				}
			}
		}
	}
	out.Users = copySlice(c.Users)
	out.Overrides = copySlice(c.Overrides)

	out.applyDefaults()
	return &out
}

// copySlice copies a list without collapsing nil and empty. The two are
// different documents — one writes `null`, the other `[]` — and a copy that
// turned one into the other would break the round-trip identity for any config
// holding an empty-but-present list.
func copySlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	return append(make([]T, 0, len(in)), in...)
}

// appended returns v appended to a fresh array, never into in's array.
//
// The one-idiom form of the rule at the top of edit.go: "add one without touching
// what a sibling holder may still be reading" is a single named operation, so a call
// site cannot half-do it. copySlice followed by append is correct but allocates twice
// — copySlice returns len == cap, so the append reallocates — and, more importantly,
// it reads as two independent steps that a future edit can separate.
//
// It is deliberately not a general-purpose helper outside this package's
// copy-on-write discipline: the cap is exact, so a second append reallocates again.
func appended[T any](in []T, v T) []T {
	out := make([]T, len(in), len(in)+1)
	copy(out, in)
	return append(out, v)
}

// DayNames are the accepted weekday spellings, lowercase, in week order.
var DayNames = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// IsDay reports whether s is one of the accepted weekday names.
func IsDay(s string) bool { return slices.Contains(DayNames, s) }
