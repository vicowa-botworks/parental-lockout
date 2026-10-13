//go:build unix

package platform

import (
	"context"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"testing"
)

// The reason this test exists: when SysProcAttr.Credential is set, the runtime calls
// setgid(cred.Gid) unconditionally before setuid(cred.Uid). A Credential built with
// only Uid set therefore does not leave the group alone — it sets group 0, so the
// "downgraded" helper runs as the session user with the root group. That is invisible
// in an audit log and still a privilege.
func TestDropPrivilegesSetsTheRealPrimaryGroup(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		t.Fatalf("unparseable uid %q: %v", u.Uid, err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		t.Fatalf("unparseable gid %q: %v", u.Gid, err)
	}
	if gid == 0 {
		t.Skip("running as a user whose primary group is 0; this host cannot distinguish the bug from the fix")
	}

	cmd := exec.Command("/bin/true")
	if err := dropPrivileges(cmd, uint32(uid)); err != nil {
		t.Fatalf("dropPrivileges: %v", err)
	}
	cred := cmd.SysProcAttr.Credential
	if cred == nil {
		t.Fatal("no Credential set; the command would run as the daemon")
	}
	if cred.Uid != uint32(uid) {
		t.Errorf("Uid = %d, want %d", cred.Uid, uid)
	}
	if cred.Gid != uint32(gid) {
		t.Errorf("Gid = %d, want %d — the runtime calls setgid unconditionally, so a zero here means the child keeps the root group", cred.Gid, gid)
	}
}

// An account that does not exist must stop the command. Falling back to gid 0 would
// be the silent version of the bug above.
func TestDropPrivilegesRefusesAnUnknownAccount(t *testing.T) {
	// 2^31-1 is not a plausible local account on any host running this.
	cmd := exec.Command("/bin/true")
	err := dropPrivileges(cmd, 2147483647)
	if err == nil {
		t.Fatal("dropPrivileges accepted a uid with no account behind it")
	}
	if !strings.Contains(err.Error(), "2147483647") {
		t.Errorf("error %q does not name the uid that could not be resolved", err)
	}
	if cmd.SysProcAttr != nil {
		t.Errorf("SysProcAttr was set despite the failed lookup: %+v", cmd.SysProcAttr)
	}
}

// The record the name service hands over is not ours to trust, and the guard below
// cannot be produced on a healthy host — so it is checked against the malformed
// record directly rather than left as a branch no test has ever entered.
func TestPrimaryGidRejectsAMalformedRecord(t *testing.T) {
	// gid_t is 32 bits, so a large value is legal and must not be rejected; only
	// something that is not a number, or is negative, is a malformed record.
	for _, gid := range []string{"", "abc", "-1", "1.5", "0x10", "1000 "} {
		if _, err := primaryGid(&user.User{Username: "kid", Uid: "1002", Gid: gid}); err == nil {
			t.Errorf("primaryGid accepted a record with Gid=%q", gid)
		}
	}
	got, err := primaryGid(&user.User{Username: "kid", Uid: "1002", Gid: "1002"})
	if err != nil || got != 1002 {
		t.Errorf("primaryGid = %d, %v; want 1002, nil", got, err)
	}
}

// Run has to surface a failed privilege drop rather than exec anyway: a helper that
// was meant to run as the session user and silently ran as root is the worst kind of
// success.
func TestExecRunnerSurfacesAFailedPrivilegeDrop(t *testing.T) {
	out, err := ExecRunner{}.Run(context.Background(), []string{"/bin/true"}, nil, 2147483647)
	if err == nil {
		t.Fatalf("a command ran as a nonexistent account (output %q)", out)
	}
	if !strings.Contains(err.Error(), "2147483647") {
		t.Errorf("error %q does not name the uid it could not resolve", err)
	}
}

// The whole point of the seam is that this combination — an account that resolves,
// with a record that does not — is now testable instead of merely defended against.
func TestDropPrivilegesRefusesAMalformedRecord(t *testing.T) {
	saved := lookupUser
	defer func() { lookupUser = saved }()
	lookupUser = func(string) (*user.User, error) {
		return &user.User{Username: "kid", Uid: "1002", Gid: "not-a-number"}, nil
	}

	cmd := exec.Command("/bin/true")
	err := dropPrivileges(cmd, 1002)
	if err == nil {
		t.Fatal("dropPrivileges accepted an account with an unparseable group")
	}
	if !strings.Contains(err.Error(), "not-a-number") {
		t.Errorf("error %q does not quote the field that is wrong", err)
	}
	if cmd.SysProcAttr != nil {
		t.Errorf("SysProcAttr was set despite the failure: %+v", cmd.SysProcAttr)
	}
}
