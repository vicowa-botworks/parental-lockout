package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// initPaths is a private config and state directory for one test.
func initPaths(t *testing.T) (configPath, stateDir string) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "etc", "plock", "config.json"), filepath.Join(dir, "var", "lib", "plock")
}

func TestInitCreatesTheAdministrator(t *testing.T) {
	cfgPath, state := initPaths(t)

	code, stdout, stderr := doInitOut(t,
		"--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("the config did not survive the round trip: %v", err)
	}
	if cfg.Admin.Username != "parent" {
		t.Errorf("Username = %q", cfg.Admin.Username)
	}
	if cfg.Admin.Password == "" {
		t.Fatal("no verifier was stored")
	}
	if !cfg.Admin.UpdatedAt.IsZero() == false {
		t.Error("UpdatedAt was not set, so nothing records when the credential changed")
	}
	if !strings.Contains(stdout, "https://") {
		t.Errorf("stdout does not tell the operator where to sign in: %q", stdout)
	}
}

// The password must not be recoverable from anything the command writes.
//
// Asserted against the raw bytes of every file the command created, not against
// the decoded struct: a field name that happens to serialise the plaintext, or a
// debug field added later, would be invisible to a struct-level check.
func TestInitNeverStoresThePasswordItself(t *testing.T) {
	cfgPath, state := initPaths(t)
	const secret = "a password long enough"

	if code, _, stderr := doInitOut(t, "--username", "parent", "--password", secret,
		"--config", cfgPath, "--state-dir", state); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Error("the config contains the password in plaintext")
	}
	// And the stored value really is a verifier this package accepts.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Parse(cfg.Admin.Password); err != nil {
		t.Errorf("the stored value is not a verifier: %v", err)
	}
	p, _ := auth.Parse(cfg.Admin.Password)
	if err := auth.CheckStoredCost(p); err != nil {
		t.Errorf("init stored a credential below the strength floor: %v", err)
	}
}

// Re-running must not silently reset the password.
//
// The failure this prevents is a provisioning script that runs twice — on a
// reimage, or because somebody re-ran the playbook — and quietly changes a
// password the parent has since written down and told nobody else.
func TestInitIsIdempotentOnlyWithForce(t *testing.T) {
	cfgPath, state := initPaths(t)
	args := []string{"--username", "parent", "--password", "a password long enough", "--config", cfgPath, "--state-dir", state}

	if code, _, _ := doInitOut(t, args...); code != exitOK {
		t.Fatalf("first run failed: exit %d", code)
	}
	first, _ := os.ReadFile(cfgPath)

	code, _, stderr := doInitOut(t, args...)
	if code == exitOK {
		t.Fatal("a second run replaced the administrator without --force")
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("the refusal does not say how to proceed: %q", stderr)
	}
	second, _ := os.ReadFile(cfgPath)
	if string(first) != string(second) {
		t.Error("the refused run still modified the config")
	}

	force := append(append([]string{}, args...), "--force")
	if code, _, stderr := doInitOut(t, force...); code != exitOK {
		t.Fatalf("with --force: exit %d: %s", code, stderr)
	}
	third, _ := os.ReadFile(cfgPath)
	if string(third) == string(first) {
		t.Error("--force did not change the credential; a fresh salt should make it differ")
	}
}

func TestInitRefusesAWeakPassword(t *testing.T) {
	cfgPath, state := initPaths(t)

	for _, weak := range []string{"", "abcdef", "12345678901"} {
		code, _, stderr := doInitOut(t, "--username", "parent", "--password", weak,
			"--config", cfgPath, "--state-dir", state)
		if code == exitOK {
			t.Errorf("%q (%d chars) was accepted", weak, len(weak))
		}
		if strings.Contains(stderr, weak) && weak != "" {
			t.Errorf("the refusal echoes the password: %q", stderr)
		}
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Error("a rejected password still created a config")
	}
	// The boundary itself: exactly the minimum is accepted.
	if code, _, stderr := doInitOut(t, "--username", "parent", "--password", "twelve char!",
		"--config", cfgPath, "--state-dir", state); code != exitOK {
		t.Errorf("a password of exactly the minimum length: exit %d: %s", code, stderr)
	}
}

// --password-stdin is the form a script should use, so what it reads has to be
// exactly what the script meant. A heredoc's trailing newline is not part of
// anybody's password, and a password ending in a space is.
func TestPasswordStdinReadsExactlyTheLine(t *testing.T) {
	for name, in := range map[string]string{
		"newline terminated": "a password long enough\n",
		"crlf terminated":    "a password long enough\r\n",
		"no terminator":      "a password long enough",
		"trailing space":     "a password long enough \n",
	} {
		cfgPath, state := initPaths(t)
		code, _, stderr := runInitWithStdin(t, in, "--username", "parent", "--password-stdin",
			"--config", cfgPath, "--state-dir", state)
		if code != exitOK {
			t.Fatalf("%s: exit %d: %s", name, code, stderr)
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := strings.TrimRight(in, "\r\n")
		if !(auth.Hasher{}).Verify(want, cfg.Admin.Password) {
			t.Errorf("%s: the stored credential is not for %q", name, want)
		}
	}
}

func TestPasswordFlagsAreExclusive(t *testing.T) {
	cfgPath, state := initPaths(t)

	// Both: which one wins would otherwise be decided by the order of two ifs.
	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--password-stdin", "--config", cfgPath, "--state-dir", state)
	if code != exitUsage {
		t.Errorf("both flags: exit %d, want %d; %s", code, exitUsage, stderr)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("the refusal does not say why: %q", stderr)
	}
	// Neither. The message is asserted as well as the code: an empty password also
	// fails the length check, so exit 2 alone cannot tell "you forgot the password"
	// from "your password is 0 characters".
	if code, _, stderr := doInitOut(t, "--username", "parent", "--config", cfgPath, "--state-dir", state); code != exitUsage {
		t.Errorf("no password: exit %d, want %d", code, exitUsage)
	} else if !strings.Contains(stderr, "a password is required") {
		t.Errorf("the no-password refusal says %q instead of naming the missing flag", stderr)
	}
	// No username.
	if code, _, stderr := doInitOut(t, "--password", "a password long enough", "--config", cfgPath, "--state-dir", state); code != exitUsage {
		t.Errorf("no username: exit %d, want %d", code, exitUsage)
	} else if !strings.Contains(stderr, "--username is required") {
		t.Errorf("the no-username refusal says %q instead of naming the missing flag", stderr)
	}
}

// A config that exists and cannot be parsed must stop the command.
//
// Overwriting it would delete every user, profile and rule on the machine because
// somebody typed the wrong password twice. "Not there" is the only state that
// means start fresh.
func TestInitRefusesToOverwriteAnUnreadableConfig(t *testing.T) {
	cfgPath, state := initPaths(t)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	const broken = `{"version":1,"settings":{"enforcement":`
	if err := os.WriteFile(cfgPath, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code == exitOK {
		t.Fatal("init replaced a config it could not read")
	}
	now, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(now) != broken {
		t.Error("the unreadable config was modified")
	}
	_ = stderr
}

// Everything init preserves about an existing document is the point of loading it
// rather than writing a new one.
func TestInitKeepsTheRestOfTheConfig(t *testing.T) {
	cfgPath, state := initPaths(t)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Profiles = []config.Profile{{ID: "schoolnight", Name: "School night", Timezone: "UTC"}}
	cfg.Users = []config.User{{Username: "casey", ProfileID: "schoolnight", Enabled: true}}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	after, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Users) != 1 || after.Users[0].Username != "casey" {
		t.Errorf("users after init = %+v, want casey still there", after.Users)
	}
	if len(after.Profiles) != 1 || after.Profiles[0].ID != "schoolnight" {
		t.Errorf("profiles after init = %+v, want schoolnight still there", after.Profiles)
	}
}

func TestInitCreatesAPrivateStateDir(t *testing.T) {
	cfgPath, state := initPaths(t)
	if code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	fi, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Errorf("state dir mode = %v, want 0700", fi.Mode().Perm())
	}
}

// A state directory somebody else made is checked, not corrected: a root CLI that
// chmods a directory it did not create is an arbitrary-chmod primitive aimed at
// whatever --state-dir says.
func TestInitRefusesALooseStateDirRatherThanChmoddingIt(t *testing.T) {
	cfgPath, state := initPaths(t)
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(state, 0o777); err != nil {
		t.Skipf("cannot chmod: %v", err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code == exitOK {
		t.Fatal("init accepted a state directory anybody can read")
	}
	fi, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() == 0o700 {
		t.Error("init chmodded a directory it did not create")
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Error("the config was written despite the state directory being refused")
	}
	_ = stderr
}

func TestInitUsageSurface(t *testing.T) {
	if code, out, _ := runCmd(t, "init", "-h"); code != exitOK || !strings.Contains(out, "--password-stdin") {
		t.Errorf("`init -h` = exit %d, out %q", code, out)
	}
	if code, out, _ := runCmd(t, "help", "init"); code != exitOK || !strings.Contains(out, "Usage: plock init") {
		t.Errorf("`help init` = exit %d, out %q", code, out)
	}
	// An unexpected positional is a usage error, not a silently ignored word.
	cfgPath, state := initPaths(t)
	// The surplus word goes last on purpose. flag.Parse stops at the first
	// non-flag argument, so putting it first makes every later flag unparsed and
	// the command fails for the wrong reason -- which is how this test was passing
	// while the check it claimed to cover was deleted.
	if code, _, stderr := runCmd(t, "init", "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state, "surplus"); code != exitUsage {
		t.Errorf("unexpected argument: exit %d, want %d; %s", code, exitUsage, stderr)
	}
	if code, _, _ := runCmd(t, "init", "--nope"); code != exitUsage {
		t.Errorf("unknown flag: exit %d, want %d", code, exitUsage)
	}
}

func TestHostAndPort(t *testing.T) {
	for listen, want := range map[string]string{
		"0.0.0.0:8443":     "localhost:8443",
		"[::]:8443":        "[::]:8443",
		":8443":            "localhost:8443",
		"192.168.1.5:8443": "192.168.1.5:8443",
		"noport":           "noport",
	} {
		if got := hostAndPort(listen); got != want {
			t.Errorf("hostAndPort(%q) = %q, want %q", listen, got, want)
		}
	}
}

// doInitOut runs the command with no stdin and captures its output.
func doInitOut(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return runInitWithStdin(t, "", args...)
}

func runInitWithStdin(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	f, err := parseInitArgs(args)
	if err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	var out, errBuf strings.Builder
	code := doInit(f, &out, &errBuf, strings.NewReader(stdin))
	return code, out.String(), errBuf.String()
}

// The state-directory guards, each asserted on its own.
//
// They are separate tests rather than a table because each refuses for a different
// reason and the reason is the point: an operator reading "is not a directory"
// needs to look at the config line, while "is a symlink" means somebody did that
// on purpose.

func TestInitRefusesASymlinkedStateDir(t *testing.T) {
	cfgPath, state := initPaths(t)
	real := filepath.Join(filepath.Dir(state), "real")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, state); err != nil {
		t.Skipf("cannot symlink: %v", err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code == exitOK {
		t.Fatal("init followed a symlink to a state directory")
	}
	if !strings.Contains(stderr, "symlink") {
		t.Errorf("the refusal does not name the symlink: %q", stderr)
	}
}

func TestInitRefusesAStateDirThatIsAFile(t *testing.T) {
	cfgPath, state := initPaths(t)
	if err := os.MkdirAll(filepath.Dir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code == exitOK {
		t.Fatal("init accepted a state directory that is a regular file")
	}
	// The message must name the path it is complaining about, not the errno from
	// some later open that happens to produce similar words.
	if !strings.Contains(stderr, state+" is not a directory") {
		t.Errorf("the refusal does not name %s as the non-directory: %q", state, stderr)
	}
}

// The ownership guard, reached without root.
//
// It only ever fires for a directory owned by somebody else, and an unprivileged
// test cannot make one, so the answer to "who are we" is what moves instead.
func TestInitRefusesAStateDirOwnedByAnotherAccount(t *testing.T) {
	cfgPath, state := initPaths(t)
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	prev := geteuid
	geteuid = func() uint32 { return prev() + 1000 }
	t.Cleanup(func() { geteuid = prev })

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code == exitOK {
		t.Fatal("init worked in a directory another account owns")
	}
	if !strings.Contains(stderr, "owned by uid") {
		t.Errorf("the refusal does not name ownership: %q", stderr)
	}
}

// The mode on MkdirAll is a request; the chmod after it is the result.
//
// Under a umask strict enough to strip the owner's own bits, the directory that
// comes out of MkdirAll is one the creator cannot even enter, so the daemon would
// fail at startup and be restarted into a loop that never locks anybody out.
func TestInitCreatesTheStateDirPrivateUnderARestrictiveUmask(t *testing.T) {
	cfgPath, state := initPaths(t)
	// The parent exists already, as /var/lib does on any real machine. Under a
	// mask that strips the owner's own bits, MkdirAll would produce an unusable
	// /var/lib as well, and the test would be about the parent chain rather than
	// about the one directory this command creates.
	if err := os.MkdirAll(filepath.Dir(state), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := forceUmask(t, 0o700)

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	restore()
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	fi, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Errorf("state dir mode = %v, want 0700", fi.Mode().Perm())
	}
}

// forceUmask sets the process umask and hands back the restorer.
//
// Returning it rather than deferring is the point: the mask has to cover exactly
// the operation under test. A `defer` that restores before the write is what made
// a umask test in internal/audit pass without ever testing anything.
func forceUmask(t *testing.T, mask int) func() {
	t.Helper()
	prev := syscall.Umask(mask)
	var once sync.Once
	restore := func() { once.Do(func() { syscall.Umask(prev) }) }
	t.Cleanup(restore)
	return restore
}

// The ancestor guards. A root CLI that creates /var/lib/plock walks through
// /var/lib, and each level it passes through is a place somebody could have put a
// symlink or a file -- the same arbitrary-write shape the leaf guard exists for,
// one directory up.

func TestInitRefusesASymlinkedAncestor(t *testing.T) {
	cfgPath, state := initPaths(t)
	root := filepath.Dir(filepath.Dir(state)) // the `var` that holds `lib/plock`
	real := filepath.Join(filepath.Dir(root), "real")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, root); err != nil {
		t.Skipf("cannot symlink: %v", err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code == exitOK {
		t.Fatal("init created a state directory through a symlinked ancestor")
	}
	if !strings.Contains(stderr, "symlink") {
		t.Errorf("the refusal does not name the symlink: %q", stderr)
	}
}

func TestInitRefusesAnAncestorThatIsAFile(t *testing.T) {
	cfgPath, state := initPaths(t)
	root := filepath.Dir(filepath.Dir(state))
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	if code == exitOK {
		t.Fatal("init walked through an ancestor that is a regular file")
	}
	if !strings.Contains(stderr, root+" exists and is not a directory") {
		t.Errorf("the refusal does not name %s: %q", root, stderr)
	}
}

// Every level this command creates has to be usable by the process that created
// it, not just the last one.
//
// Under a mask that strips the owner's own bits, MkdirAll's mode comes out as 0000
// for the intermediate directory and the next mkdir inside it fails with
// "permission denied" -- about a directory this command made a moment earlier,
// which reads like somebody else's misconfiguration. Creating one level at a time
// and tightening it before descending is what makes a fresh machine work.
func TestInitCreatesEveryMissingLevelUnderARestrictiveUmask(t *testing.T) {
	cfgPath, state := initPaths(t)
	restore := forceUmask(t, 0o700)
	code, _, stderr := doInitOut(t, "--username", "parent", "--password", "a password long enough",
		"--config", cfgPath, "--state-dir", state)
	restore()
	if code != exitOK {
		t.Fatalf("init could not create its own directory tree under a strict umask: exit %d: %s", code, stderr)
	}
	for _, dir := range []string{filepath.Dir(filepath.Dir(state)), filepath.Dir(state), state, filepath.Dir(cfgPath)} {
		fi, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		if fi.Mode().Perm() != 0o700 {
			t.Errorf("%s mode = %v, want 0700", dir, fi.Mode().Perm())
		}
	}
}

// The refusal must not promise something the daemon does not do.
//
// The message used to end with "Every existing admin session ends when you do." Nothing
// ends them: Sessions lives in the daemon's memory, plock init writes a file and exits,
// and no code consumes Admin.UpdatedAt. That is printed at the moment a parent is most
// likely rotating the password *because* they believe a session is compromised, and it
// tells them the mitigation happened.
//
// The assertion is on the shape of the claim rather than its exact wording: it must name
// the restart as the thing that ends sessions, and it must not say sessions end on their
// own.
func TestTheReplaceRefusalSaysHowSessionsActuallyEnd(t *testing.T) {
	cfgPath, state := initPaths(t)
	if code, _, stderr := doInitOut(t, "--username", "parent",
		"--password", "a password long enough", "--config", cfgPath, "--state-dir", state); code != exitOK {
		t.Fatalf("first init failed: %d %s", code, stderr)
	}

	code, _, stderr := doInitOut(t, "--username", "parent",
		"--password", "a different password long enough", "--config", cfgPath, "--state-dir", state)
	if code == exitOK {
		t.Fatal("replacing an existing administrator without --force succeeded")
	}
	if !strings.Contains(stderr, "restart") {
		t.Errorf("the refusal does not tell the operator how to end existing sessions:\n%s", stderr)
	}
	for _, claim := range []string{"session ends", "sessions end", "session is ended", "sessions are ended"} {
		if strings.Contains(strings.ToLower(stderr), claim) {
			t.Errorf("the refusal claims %q, which no code does; sessions survive until the daemon restarts", claim)
		}
	}
}
