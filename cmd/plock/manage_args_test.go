package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// A flag written after `--` stops being a flag, and a management command that ignores the
// leftover silently edits the wrong file.
//
// This is the dangerous half of the separator fix. Making `--` usable means more parents
// reach for it, and every command in manage.go parsed its arguments through a helper that
// never looked at what flag left behind. So `plock user rm -- bob --config /tmp/test.json`
// adopted bob, dropped --config back on the floor, and went to the default path: the real
// /etc/plock/config.json. The operator reads "no such user" from their scratch file and
// runs it again without the flag.
//
// The guard is in cmdFlags.parse rather than at each call site because all fifteen
// management commands share it, and a guard that has to be remembered fifteen times is a
// guard that will be forgotten once.
func TestManagementCommandsRefuseAnArgumentTheyDidNotClaim(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"user rm with a swallowed --config", []string{"user", "rm", "--", "bob", "--config", "x.json"}},
		{"profile show with a swallowed --config", []string{"profile", "show", "--", "p", "--config", "x.json"}},
		{"user enable with a swallowed --config", []string{"user", "enable", "--", "bob", "--config", "x.json"}},
		{"profile rm with a stray word", []string{"profile", "rm", "p", "q"}},
		{"profile list with a stray word", []string{"profile", "list", "surprise"}},
		// user assign is the one command with two positionals. It takes them with two
		// TakeFirst calls, so both are claimed before parse and the guard must not fire.
		// Its third word still must.
		{"user assign with a third word", []string{"user", "assign", "bob", "p", "extra"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errOut := runCmd(t, tc.args...)
			if code == exitOK {
				t.Fatalf("`plock %s` was accepted; an unclaimed argument must be an error\nstderr:\n%s",
					strings.Join(tc.args, " "), errOut)
			}
			if !strings.Contains(errOut, "unexpected argument") {
				t.Errorf("stderr does not name the unclaimed argument:\n%s", errOut)
			}
		})
	}
}

// The guard must not cost the ordinary invocation anything.
//
// A refusal added to a shared parse is only safe if the commands that were already correct
// still work, and the way this one goes wrong is by rejecting a legitimate trailing
// argument that some command was relying on.
func TestManagementCommandsStillAcceptTheirNormalForm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{"version":1,
 "settings":{"enforcement":true,"pollSeconds":30,"lockBackends":["logind"]},
 "profiles":[{"id":"p","name":"P","timezone":"UTC","rules":[]}],
 "users":[{"username":"bob","profileID":"p","enabled":true}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := runCmd(t, "profile", "show", "p", "--config", path)
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitOK, out, errOut)
	}
	if !strings.Contains(out, "p") {
		t.Errorf("profile show printed nothing useful:\n%s", out)
	}

	// The separator form, with every flag ahead of it, is the invocation the fix exists
	// for and must still work end to end.
	code, _, errOut = runCmd(t, "profile", "show", "--config", path, "--", "p")
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstderr:\n%s", code, exitOK, errOut)
	}

	// Two positionals is the shape a naive "no leftovers" guard breaks, so it is asserted
	// here rather than assumed: user assign claims both before parse ever sees them.
	code, _, errOut = runCmd(t, "user", "assign", "bob", "p", "--config", path)
	if code != exitOK {
		t.Fatalf("user assign with two positionals: exit %d\nstderr:\n%s", code, errOut)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("the refused runs must not have touched the config: %v", err)
	}
	if len(cfg.Users) != 1 || cfg.Users[0].Username != "bob" {
		t.Errorf("users = %+v, want bob untouched by the refusal tests", cfg.Users)
	}
}
