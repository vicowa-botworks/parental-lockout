// Package config owns the single JSON document that is the whole persistent
// state of the application: who the administrator is, which lockout profiles
// exist, which child account is assigned to which profile, and any temporary
// overrides in force.
//
// Everything else in the program reads and writes that document through this
// package, so the on-disk rules — atomic replacement, mode 0600, refusal of a
// version it does not understand — are stated once here rather than being
// re-derived at every call site.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
// applyDefaults, and naming them here would only invite the two to drift.
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
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var raw configJSON
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the config object")
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
// application off — the one failure mode this program must not have.
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
//
// A custom unmarshaler steps outside the decoder's DisallowUnknownFields
// setting, so the strictness is re-established here rather than quietly lost
// for the one section a parent is most likely to hand-edit.
func (s *Settings) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var raw settingsJSON
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the settings object")
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

// Rule is a wall-clock window on a set of weekdays. End earlier than or equal
// to Start means the window crosses midnight and belongs to the starting day.
type Rule struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Days    []string `json:"days"`
	Start   string   `json:"start"`
	End     string   `json:"end"`
	Enabled bool     `json:"enabled"`
}

// User assigns exactly one profile to one local account.
type User struct {
	Username  string `json:"username"`
	ProfileID string `json:"profileID"`
	Enabled   bool   `json:"enabled"`
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
// omitted key and an explicit false are the same value; the distinction is made
// in UnmarshalJSON, where the difference is still visible.
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

// applyDefaults fills in what a decoded or hand-built document may leave out.
// It is called by Load after decoding and by Save before encoding, which is
// what makes Save-then-Load an identity: the bytes on disk are always the
// normalized form, and normalizing them again changes nothing.
func (c *Config) applyDefaults() {
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	c.Settings.applyDefaults()
}

// normalized returns a copy with defaults applied, leaving the receiver alone:
// Save must not silently mutate the caller's value.
func (c *Config) normalized() *Config {
	out := *c
	out.Settings.LockBackends = append([]string(nil), c.Settings.LockBackends...)
	out.applyDefaults()
	return &out
}

// DayNames are the accepted weekday spellings, lowercase, in week order.
var DayNames = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// IsDay reports whether s is one of the accepted weekday names.
func IsDay(s string) bool {
	for _, d := range DayNames {
		if s == d {
			return true
		}
	}
	return false
}
