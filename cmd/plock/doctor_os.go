package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// The operating-system side of the doctor checks: the calls that touch the filesystem, the
// certificate store, and the init system. They are separate from the checks themselves so
// every check stays a pure function of data a test can state, which is the difference
// between a diagnostic test that asserts on a machine's actual state and one that asserts on
// what the diagnostic concluded.

// certValidity reads a certificate's validity window without deciding whether it is good.
//
// The judgement is the check's; this only reports. Parsing stops at the first certificate in
// the file, which is what a chain file wants: the leaf is first by convention, and it is the
// leaf the browser checks.
func certValidity(path string) (notBefore, notAfter time.Time, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%s holds no PEM block", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%s: %w", path, err)
	}
	return cert.NotBefore, cert.NotAfter, nil
}

// systemdState asks the init system whether the plock service is running.
//
// It returns `known=false` when systemctl is not on this machine, which is a different
// answer from "inactive" and has to stay different: on a box running a different supervisor,
// "inactive" would be a fabricated finding and the operator would go looking for a service
// that was never there.
//
// systemctl exits 1 for "inactive" and 3 for... several things, so the state is read from
// stdout rather than the exit code. `is-active` prints the state word and the word is the
// answer.
func systemdState(run platform.Runner) (state string, known bool, err error) {
	if run == nil {
		return "", false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// uid 0 and a nil env: systemctl is asked as the invoking user, and `is-active` is
	// answerable by anyone. Inheriting the environment is what makes this work when the
	// operator has set SYSTEMD_ADDRESS to reach a container's init system.
	out, err := run.Run(ctx, []string{"systemctl", "is-active", "plock"}, nil, 0)
	if err != nil {
		// A missing binary is the only error that means "no systemd here", and it is
		// recognised by identity. Matching prose matched the stdlib's sentence
		// ("executable file not found in $PATH") while ExecRunner deliberately rewrites
		// it to "systemctl: not found in PATH" -- so on a box with no systemctl this
		// returned a *failure to ask* and doctor warned "could not ask systemd" about a
		// machine that simply has no init system, sending the operator to look for a
		// broken bus instead of the absent one.
		if errors.Is(err, platform.ErrBinaryNotFound) {
			return "", false, nil
		}
		// systemctl itself exits non-zero to report a non-active state, and the state word is
		// on stdout either way, so the state is read first and the error only decides whether
		// the answer is trustworthy.
		if word := parseActiveState(out); word != "" {
			return word, true, nil
		}
		return "", true, err
	}
	word := parseActiveState(out)
	if word == "" {
		return "", true, fmt.Errorf("systemctl is-active said %q", strings.TrimSpace(string(out)))
	}
	return word, true, nil
}

// parseActiveState pulls the state word out of `systemctl is-active` output.
func parseActiveState(out []byte) string {
	for _, line := range strings.Split(string(out), "\n") {
		w := strings.TrimSpace(line)
		switch w {
		case "active", "inactive", "activating", "deactivating", "failed", "unknown":
			return w
		}
	}
	return ""
}

// isNotFound reports whether an error means the program could not be executed at all.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "no such file or directory")
}
