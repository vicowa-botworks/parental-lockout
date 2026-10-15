package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// #11's acceptance criteria: "--json on every read command ... exit code 0 on success and
// non-zero on failure with the error on stderr as JSON too."
//
// This is the contract test for that sentence, and it walks every command that takes
// --json rather than testing one of them. The reason is what this repo had before: status
// wrote its JSON error to stdout while every other command wrote text to stderr, each
// choice defensible in its own file and together meaning no script could rely on either.
// A per-command test would have passed on both halves of that.
//
// Three things are asserted, and the third catches the interesting bug:
//   - the failure is JSON, on stderr, with a code and a message;
//   - the exit code is the operational one, not the usage one;
//   - stdout carries NO document.
//
// That last one is what makes `plock status --json 2>&1 | jq` work. With the error on
// stdout, the merged stream is a JSON document interleaved with a human line and jq parses
// neither; with it on both streams, jq reads the first document and errors on the second.
// Empty stdout plus exit 1 is the only pair where the merged stream is exactly one
// document and the absence of a report is itself a signal.
func TestEveryJSONCommandReportsItsFailureAsJSONOnStderr(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.json")
	// A document that reads as bytes and then refuses to decode, so every command fails at
	// the same place -- the config load -- and any difference in what it reports is the
	// contract, not a different fault.
	if err := os.WriteFile(bad, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatalf("write %s: %v", bad, err)
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"status", []string{"status", "--json"}},
		{"profile_list", []string{"profile", "list", "--json"}},
		{"profile_show", []string{"profile", "show", "any-profile", "--json"}},
		{"user_list", []string{"user", "list", "--json"}},
		{"override_list", []string{"override", "list", "--json"}},
		{"lock", []string{"lock", "casey", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(append(append([]string{}, tc.args...), "--config", bad), &stdout, &stderr)

			if code != exitFailure {
				t.Errorf("exit code = %d, want %d (an operational failure, not a usage error)",
					code, exitFailure)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout carries a document on a failed run, so a merged 2>&1 stream is two documents:\n%s",
					stdout.String())
			}

			var doc struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &doc); err != nil {
				t.Fatalf("stderr is not one JSON document (%v):\n%s", err, stderr.String())
			}
			if doc.Error == "" {
				t.Error(`"error" is empty: a --json failure with no code is the same lie as "error": null on a success`)
			}
			if doc.Message == "" {
				t.Error(`"message" is empty: the human-readable form has to survive into the machine grammar`)
			}
			if !strings.Contains(doc.Message, "version") {
				t.Errorf("message does not describe the actual fault: %q", doc.Message)
			}
		})
	}
}

// The other half of the same sentence. Asserted because a change that moved errors to
// stderr by moving *everything* to stderr would satisfy the test above and break every
// consumer that reads the report off stdout.
func TestASuccessfulJSONRunKeepsStderrEmpty(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"status", []string{"status", "--json"}},
		{"profile_list", []string{"profile", "list", "--json"}},
		{"user_list", []string{"user", "list", "--json"}},
		{"override_list", []string{"override", "list", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(append(append([]string{}, tc.args...), "--config", writeConfig(t)), &stdout, &stderr); code != exitOK {
				t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("a successful run wrote to stderr: %q", stderr.String())
			}
			if !json.Valid(stdout.Bytes()) {
				t.Errorf("stdout is not valid JSON:\n%s", stdout.String())
			}
		})
	}
}

// A usage error is not an operational failure and must not be dressed up as one: it means
// the command line was wrong, which is a human reading the usage text, and exit 2 already
// carries that without needing a grammar to parse.
func TestUsageErrorsStayPlainTextEvenWithJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"status", "--json", "--nonsense-flag"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if json.Valid(stderr.Bytes()) {
		t.Errorf("a usage error came back as JSON; exit 2 is the machine signal for it: %q", stderr.String())
	}
	if stderr.Len() == 0 {
		t.Error("a usage error said nothing at all")
	}
}

// The codes are part of the documented surface, so renaming one is a breaking change and
// the list belongs in a test as well as in docs/cli.md. Written as documented -> actual so
// a rename fails here instead of silently redefining the promise.
func TestFailureCodesAreTheDocumentedSet(t *testing.T) {
	for documented, actual := range map[string]string{
		"config_unreadable":   codeConfigUnreadable,
		"config_unstatable":   codeConfigUnstatable,
		"config_unwritable":   codeConfigUnwritable,
		"no_backend":          codeNoBackend,
		"sessions_failed":     codeSessionsFailed,
		"no_such_user":        codeNoSuchUser,
		"no_lockable_session": codeNoLockableSession,
		"lock_failed":         codeLockFailed,
		"no_such_profile":     codeNoSuchProfile,
		"output_failed":       codeOutputFailed,
	} {
		if actual != documented {
			t.Errorf("code %q is implemented as %q; docs/cli.md promises the first", documented, actual)
		}
	}
}
