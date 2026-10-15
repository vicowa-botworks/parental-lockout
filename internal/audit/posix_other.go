//go:build !linux

package audit

import "os"

// openAppend opens the audit file for appending. This platform has no
// O_NOFOLLOW, so the symlink guard cannot be applied here — which is why
// ownerIsRoot refuses below rather than reporting success. A platform that
// cannot answer the tamper questions is a platform that must not write the
// trail, not one that writes it unchecked.
func openAppend(path string, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, mode)
}

// openRead opens the audit file to read it back. This platform has no O_NOFOLLOW, so the
// symlink guard cannot be applied; the reader therefore checks the shape it actually
// opened (reader.go) and refuses anything that is not a regular file, which is the guard
// that survives without POSIX flags.
func openRead(path string) (*os.File, error) {
	return os.Open(path)
}

// ownerIsWriter has no answer on this platform. Reporting that is deliberate: the
// guard fails closed, so a future port has to implement this instead of quietly
// losing the rule that the trail belongs to root.
func ownerIsWriter(os.FileInfo) (bool, error) {
	return false, errOwnershipUnsupported
}

// openDir opens a directory so its mode can be set on the descriptor.
func openDir(path string) (*os.File, error) {
	return os.Open(path)
}

// setDirMode sets the mode on a directory just created inside parent.
func setDirMode(_ *os.File, name string, _ os.FileMode) error {
	return os.Chmod(name, 0o700)
}

// tightenMode has nothing to do here: this platform has no POSIX permission bits
// to strip, so declining silently is accurate rather than a swallowed failure.
func tightenMode(*os.File, os.FileMode) error {
	return nil
}
