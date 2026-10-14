//go:build unix

package platform

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// dropPrivileges makes the child run as uid before it executes anything.
//
// The gid is not optional here, and that is a property of the syscall wrapper rather
// than a choice: when SysProcAttr.Credential is set, the runtime calls
// setgid(cred.Gid) unconditionally before setuid(cred.Uid). Leaving Gid at its zero
// value therefore does not mean "leave the group alone" — it means setgid(0), so the
// helper would run as the session user with the root group, keeping whatever a
// group-0 file or socket grants. The uid looks downgraded in an audit log and the
// privilege is still there.
//
// So the primary group is looked up, and a lookup that fails stops the command
// rather than falling back to a guess. Running a desktop command with the wrong
// group is the kind of failure that shows up as "the lock silently did nothing"
// three weeks later.
// lookupUser is the name service, behind a variable for the same reason exec is
// behind Runner: it is an external dependency whose failure shapes this package has
// to handle, and "the account exists but its record is malformed" cannot be produced
// on a healthy host. A guard that has never fired is a guard that may not fire.
var lookupUser = user.LookupId

func dropPrivileges(cmd *exec.Cmd, uid uint32) error {
	u, err := lookupUser(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return fmt.Errorf("cannot run anything as uid %d: no such account (%v)", uid, err)
	}
	gid, err := primaryGid(u)
	if err != nil {
		return err
	}

	// Supplementary groups are cleared, which is the runtime's default when
	// Credential is set. A desktop helper needs the user's own bus address, not the
	// twelve groups they picked up from a VPN or a docker install, and a group the
	// session does not expect can only widen what the command can reach.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uid, Gid: uint32(gid)},
	}
	return nil
}

// primaryGid reads the account's group out of the record the name service handed us.
//
// Split out because the failure it guards — a passwd entry whose gid field is not a
// number — cannot be produced on a healthy host, and a guard nobody has ever seen
// fire is a guard that may not fire. Taking *user.User makes it checkable against the
// malformed record directly.
func primaryGid(u *user.User) (uint32, error) {
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("account %s (uid %s) has an unparseable primary group %q", u.Username, u.Uid, u.Gid)
	}
	return uint32(gid), nil
}
