package main

import (
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The `--` escape hatch, pinned at the level that was broken.
//
// internal/cli has always had a TakeFirst-level test, and it passed the whole time while
// the documented invocation was refused by every one of the 17 call sites. The reason is
// the shape of the bug rather than its size: the unit test asserted that a token after
// `--` is a *value*, and the command then treated "no positional, and leftovers in
// flag.Args()" as proof the operator mistyped. Both halves are defensible; together they
// made `plock unlock --minutes 5 -- casey` unusable. Only a test that runs the command
// can see the seam between them, which is why this file exists.
func TestUnlockAcceptsTheUsernameAfterTheSeparator(t *testing.T) {
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)
	path := unlockConfig(t)

	code, out, errOut := runCmd(t, "unlock", "--minutes", "30", "--reason", "film night",
		"--by", "pat", "--config", path, "--", "casey")
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitOK, out, errOut)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config after unlock: %v", err)
	}
	if len(cfg.Overrides) != 1 {
		t.Fatalf("got %d overrides, want 1: %+v", len(cfg.Overrides), cfg.Overrides)
	}
	got := cfg.Overrides[0]
	if got.Username != "casey" {
		t.Errorf("override username = %q, want casey", got.Username)
	}
	// The flags before the separator still parse. The separator ends flag parsing where
	// it stands; it does not retroactively turn the earlier ones into words.
	if got.Reason != "film night" || got.By != "pat" {
		t.Errorf("override = %+v, want the reason and --by from before the separator", got)
	}
	if !got.Until.Equal(now.Add(30 * time.Minute)) {
		t.Errorf("until = %v, want %v", got.Until, now.Add(30*time.Minute))
	}
}

// The case the separator exists for: an account whose name starts with a dash.
//
// Without `--` this is unreachable by construction -- flag would read "-x" as a flag and
// TakeFirst would never offer it as the positional. It is unusual, but an unusual name
// that cannot be unlocked is a parent locked out of their own tool, and the fix has to be
// provably about the name rather than about the flag order that happens to work.
func TestUnlockAcceptsAUsernameThatStartsWithADash(t *testing.T) {
	now := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	withUnlockClock(t, now)
	path := unlockConfig(t, "-x")

	code, out, errOut := runCmd(t, "unlock", "--minutes", "5", "--reason", "dash name", "--config", path, "--", "-x")
	if code != exitOK {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitOK, out, errOut)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config after unlock: %v", err)
	}
	if len(cfg.Overrides) != 1 || cfg.Overrides[0].Username != "-x" {
		t.Fatalf("overrides = %+v, want one for the account named \"-x\"", cfg.Overrides)
	}
}

// The refusal after an adopted positional names the extra word, not a count.
//
// `plock unlock -- casey anna` used to reach a different branch entirely -- "expected one
// username, got 2: [casey]" -- because the separator suppressed the positional and the +1
// then counted a username that was not there. With the separator protecting the positional
// the honest refusal is the one that names both words.
func TestUnlockCountsTheUsernamesItWasGiven(t *testing.T) {
	path := unlockConfig(t)

	code, _, errOut := runCmd(t, "unlock", "--config", path, "--", "casey", "anna")
	if code == exitOK {
		t.Fatal("two usernames were accepted")
	}
	// The second token is reported as the extra word, naming the username it decided was
	// the real one. It must not be counted as a third username: the old refusal added one
	// for a username that was never there.
	if !strings.Contains(errOut, `unexpected argument "anna" after username "casey"`) {
		t.Errorf("stderr does not name the extra word and the adopted username:\n%s", errOut)
	}
	if strings.Contains(errOut, "got 3") || strings.Contains(errOut, "got 2") {
		t.Errorf("the refusal invents usernames:\n%s", errOut)
	}
}

// A separator with nothing after it is not a username.
//
// Adopting the first post-separator token must not invent one from an empty tail, or
// `plock unlock --` would proceed with a blank account name.
func TestUnlockDoesNotInventAUsernameFromAnEmptySeparator(t *testing.T) {
	path := unlockConfig(t)

	code, _, errOut := runCmd(t, "unlock", "--minutes", "5", "--config", path, "--")
	if code == exitOK {
		t.Fatal("`unlock --` was accepted; an empty tail is not a username")
	}
	if !strings.Contains(errOut, "a username is required") {
		t.Errorf("stderr does not ask for a username:\n%s", errOut)
	}
}
