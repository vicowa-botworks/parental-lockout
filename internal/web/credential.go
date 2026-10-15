package web

import (
	"context"
	"errors"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
)

// The admin credential is a row in the config file, and the config file is edited by a
// different process than the one that authenticates against it. Without anything here,
// the daemon's view of the password is whatever it read at startup: `plock init --force`
// would write a new verifier that the web server never consults, so the new password
// would not log in and the old one still would.
//
// That is the incident path. A parent who has reason to believe an admin session is
// compromised reaches for exactly that command, and a rotation that neither revokes the
// live session nor retires the password it was stolen against is a mitigation that did
// not happen.

// errNoAuthenticator is the server having no way to check a password at all: no Config
// seam to read one from and no Auth handed to it. It is distinct from
// auth.ErrNoCredential, which is an admin record that exists but carries no verifier --
// one is a daemon assembled wrong, the other is a machine where `plock init` has not run.
var errNoAuthenticator = errors.New("no authenticator is configured")

// adminCredential is the credential the daemon should be accepting right now.
//
// It goes through the same Config seam the pages use rather than a second path to the
// file, because two ways of reading one credential is how the login page and the
// dashboard end up describing different machines.
func (s *Server) adminCredential() (username, verifier string, err error) {
	if s.Config == nil {
		return "", "", errors.New("no configuration source is wired")
	}
	cfg, err := s.Config()
	if err != nil {
		return "", "", err
	}
	if cfg == nil {
		return "", "", errors.New("the configuration source returned no configuration")
	}
	return cfg.Admin.Username, cfg.Admin.Password, nil
}

// authenticator answers with the credential to check this attempt against.
//
// A server with no Config seam keeps using the Auth it was handed. That is not a
// fallback for convenience: a caller that never gave the web layer a way to re-read the
// config has no rotated credential to notice, and inventing one here would mean the
// login page silently depends on a source it was never wired to.
func (s *Server) authenticator() (auth.Authenticator, error) {
	if s.Config == nil {
		if s.Auth == nil {
			return nil, errNoAuthenticator
		}
		return s.Auth, nil
	}
	username, verifier, err := s.adminCredential()
	if err != nil {
		return nil, err
	}
	return auth.PasswordAuthenticator{Username: username, Verifier: verifier}, nil
}

// watchCredential ends every session when the stored verifier changes, and keeps the
// login path pointed at the current one.
//
// The comparison is on the verifier, not on Admin.UpdatedAt, and that choice is the
// whole design. A timestamp is a promise the writer has to keep, and the write path
// rewrites the whole document: a UI save that touched nothing else would bump it and
// log the operator out of their own session mid-edit. The verifier is the thing the
// rotation actually changes, so "it moved" and "the password is different" cannot drift
// apart.
//
// The first read only sets the baseline. Revoking on startup would throw away every
// session on every daemon restart, which is a self-inflicted outage dressed as hygiene
// -- and it would also mean the watcher's first act depends on how fast the config file
// happens to be readable.
//
// Polling rather than a file watch: the config is one small file written by `plock init`
// and by this daemon's own future write path, and inotify on a file that is replaced
// rather than truncated means re-arming a watch on every write anyway. A poll interval
// is one number an operator can reason about; a watch that silently stopped firing after
// a rename is not.
func (s *Server) watchCredential(ctx context.Context, every time.Duration) {
	seen := ""
	have := false
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		_, verifier, err := s.adminCredential()
		if err != nil {
			// A config that cannot be read right now is not evidence the credential
			// changed. Revoking here would turn a transient read error -- the file is
			// being replaced, the disk hiccuped -- into a mass logout, and the operator
			// would have no way to tell that apart from somebody rotating on purpose.
			s.logf("web: admin credential could not be re-read: %v", err)
			continue
		}
		if !have {
			seen, have = verifier, true
			continue
		}
		if verifier == seen {
			continue
		}
		n := s.Sessions.Count()
		s.Sessions.RevokeAll()
		seen = verifier
		// Counted before the revoke, because after it the number is always zero and the
		// log line would say the watcher did something it did not.
		s.logf("web: admin credential changed; %d session(s) ended", n)
	}
}
