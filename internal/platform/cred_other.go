//go:build !unix

package platform

import (
	"errors"
	"os/exec"
)

// errNoPrivilegeDrop is reported rather than ignored. A backend that meant to run
// something as the session user and quietly ran it as root instead would be the
// worst kind of success: the lock works, the audit line names the right account, and
// a root-owned process is sitting inside someone else's desktop session.
var errNoPrivilegeDrop = errors.New("this platform cannot run a command as another user")

func dropPrivileges(cmd *exec.Cmd, uid uint32) error {
	return errNoPrivilegeDrop
}
