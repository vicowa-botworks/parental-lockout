package config

import (
	"os"
	"strings"
	"testing"
)

// The command backend is the only configured backend that runs a program this binary does
// not ship, so the two fields that name it are a security-relevant pair rather than a
// cosmetic one. Each case below is a document an operator could plausibly write and that
// must not load quietly into a daemon that then does something other than what they meant.
func TestValidateGatesTheCommandBackendFields(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		argv     []string
		backends []string
		want     string // "" means the document must validate
		absent   string // substring that must NOT appear
	}{
		{
			name:     "named and given an absolute path",
			backends: []string{"command"},
			path:     "/usr/local/bin/plock-locktest",
			want:     "",
		},
		{
			name:     "named with argv",
			backends: []string{"command"},
			path:     "/usr/local/bin/plock-locktest",
			argv:     []string{"--session", "%SESSION%"},
			want:     "",
		},
		{
			name:     "named but no path",
			backends: []string{"command"},
			want:     `lockCommandPath: required when lockBackends names "command"`,
		},
		{
			// The dangerous one, because it reads as specific. A relative path resolves
			// against the daemon's working directory, which under systemd is "/" and
			// under `make run-dev` is whatever shell the developer was standing in.
			name:     "named with a relative path",
			backends: []string{"command"},
			path:     "./locktest.sh",
			want:     "must be an absolute path",
		},
		{
			// A bare name is the same class and the rule #53 established for the shipped
			// lock binaries: refused, never resolved through PATH.
			name:     "named with a bare program name",
			backends: []string{"command"},
			path:     "locktest.sh",
			want:     "must be an absolute path",
		},
		{
			name:     "configured but not named",
			backends: []string{"logind", "kde"},
			path:     "/usr/local/bin/plock-locktest",
			want:     `set but lockBackends does not name "command"`,
		},
		{
			name:     "argv without the backend",
			backends: []string{"logind"},
			argv:     []string{"--now"},
			want:     `set but lockBackends does not name "command"`,
		},
		{
			name:     "nothing set, nothing named",
			backends: []string{"logind", "kde", "gnome"},
			want:     "",
		},
		{
			// Whitespace is not a path. Trimming before the check is what turns " " from
			// an apparently-present value into the missing-path message an operator can act on.
			name:     "named with a whitespace path",
			backends: []string{"command"},
			path:     "   ",
			want:     "required when lockBackends",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			c.Settings.LockBackends = tc.backends
			c.Settings.LockCommandPath = tc.path
			c.Settings.LockCommandArgv = tc.argv

			err := c.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Validate rejected a document that should pass: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate accepted a document it should refuse (%s); want %q", tc.name, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate said %v; want it to say %q", err, tc.want)
			}
			if tc.absent != "" && strings.Contains(err.Error(), tc.absent) {
				t.Errorf("Validate also said %q, which belongs to a different field: %v", tc.absent, err)
			}
		})
	}
}

// The refusal must name the field the operator has to edit, not the backend list. A
// message that blames lockBackends sends them to the wrong line of the file.
func TestTheCommandPathRefusalNamesTheFieldToEdit(t *testing.T) {
	c := valid()
	c.Settings.LockBackends = []string{"logind", "command"}
	c.Settings.LockCommandPath = "locktest.sh"

	err := c.Validate()
	if err == nil {
		t.Fatal("Validate accepted a bare program name")
	}
	msg := err.Error()
	if !strings.Contains(msg, "settings.lockCommandPath") {
		t.Errorf("the refusal does not name settings.lockCommandPath, so the operator cannot find the line: %v", msg)
	}
	// It must also quote the offending value: "must be an absolute path" alone does not
	// distinguish a typo'd path from an empty one.
	if !strings.Contains(msg, "locktest.sh") {
		t.Errorf("the refusal does not show the value it rejected: %v", msg)
	}
}

// A document that passes Validate must survive Save and Load unchanged, or the round trip
// through the file silently drops the backend the operator configured.
func TestTheCommandFieldsRoundTrip(t *testing.T) {
	c := valid()
	c.Settings.LockBackends = []string{"logind", "command"}
	c.Settings.LockCommandPath = "/usr/local/bin/plock-locktest"
	c.Settings.LockCommandArgv = []string{"--session", "%SESSION%", "--immediate"}

	path := t.TempDir() + "/config.json"
	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Settings.LockCommandPath != c.Settings.LockCommandPath {
		t.Errorf("LockCommandPath = %q after the round trip, want %q",
			got.Settings.LockCommandPath, c.Settings.LockCommandPath)
	}
	if strings.Join(got.Settings.LockCommandArgv, "|") != strings.Join(c.Settings.LockCommandArgv, "|") {
		t.Errorf("LockCommandArgv = %v after the round trip, want %v",
			got.Settings.LockCommandArgv, c.Settings.LockCommandArgv)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("the document Validate accepted is not the document Load hands back: %v", err)
	}
}

// An unknown key in this position must be refused rather than ignored. A misspelled
// lockCommandPath would otherwise produce a daemon with no command backend and no message
// -- and unlike most fields, "absent" here is not a safe default, because lockBackends
// already named a backend that then does not exist.
func TestAnUnknownCommandKeyIsRefused(t *testing.T) {
	for _, key := range []string{"lockCommand", "lockCommandArgs", "commandPath", "lockCommandPathArgv"} {
		err := strictDecode([]byte(`{"lockBackends":["command"],"`+key+`":"/usr/local/bin/x"}`), new(Settings))
		if err == nil {
			t.Errorf("strictDecode accepted %q, so a misspelled key would configure nothing silently", key)
		} else if !strings.Contains(err.Error(), key) {
			t.Errorf("the refusal for %q does not name the offending key: %v", key, err)
		}
	}
}

// The mirror of the above, and the reason the loop above is not over-broad: Go matches
// JSON keys to struct fields case-insensitively, so "lockcommandPath" is not a typo that
// silently configures nothing -- it configures the field. Asserting that here keeps a
// future reader from "fixing" the case-insensitive match as if it were a hole, and keeps
// the refusal test honest about what it actually refuses: unknown names, not odd ones.
func TestACaseVariantOfTheCommandKeyStillLands(t *testing.T) {
	var s Settings
	if err := strictDecode([]byte(`{"lockBackends":["command"],"lockcommandPath":"/usr/local/bin/x"}`), &s); err != nil {
		t.Fatalf("strictDecode refused a case variant Go accepts: %v", err)
	}
	if s.LockCommandPath != "/usr/local/bin/x" {
		t.Errorf("LockCommandPath = %q, want the value to have landed", s.LockCommandPath)
	}
}

// Two occurrences of the same key is its own hazard, and this file already refuses it for
// "enforcement". Asserted through Load rather than strictDecode on a bare Settings: the
// duplicate scan lives in Config.UnmarshalJSON and reaches nested objects by walking a
// token stack, so calling the inner helper directly would test a path the daemon never
// takes and would report a hole that does not exist.
func TestADuplicateCommandPathKeyIsRefused(t *testing.T) {
	path := t.TempDir() + "/config.json"
	body := `{"version":1,"admin":{"username":"root","verifier":"x"},` +
		`"settings":{"lockBackends":["command"],` +
		`"lockCommandPath":"/usr/local/bin/first","lockCommandPath":"/usr/local/bin/second"}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("a duplicated lockCommandPath loaded without error; one of the two paths is now invisible")
	}
	if !strings.Contains(err.Error(), "lockCommandPath") {
		t.Errorf("the refusal does not name the duplicated key: %v", err)
	}
}

// And the same for argv, because the two fields are a pair and only one of them was
// written with a scan in mind.
func TestADuplicateCommandArgvKeyIsRefused(t *testing.T) {
	path := t.TempDir() + "/config.json"
	body := `{"version":1,"admin":{"username":"root","verifier":"x"},` +
		`"settings":{"lockBackends":["command"],"lockCommandPath":"/usr/local/bin/x",` +
		`"lockCommandArgv":["--a"],"lockCommandArgv":["--b"]}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("a duplicated lockCommandArgv loaded without error")
	} else if !strings.Contains(err.Error(), "lockCommandArgv") {
		t.Errorf("the refusal does not name the duplicated key: %v", err)
	}
}
