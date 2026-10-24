package web

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/auth"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The admin password had no screen. `config.Settings` carried the credential, the watcher in
// credential.go already ended every session when it changed, and the only way to change it was a
// root shell and a JSON editor -- on the one secret whose entire purpose is to not be root. #99
// made this slice conditional on session revocation and that mechanism shipped long ago
// (TestARotatedCredentialEndsSessionsMintedUnderTheOldOne), so what was missing was the form.
//
// It is its own route rather than a field on POST /settings for two reasons that both outlive the
// form. A credential riding along with the enforcement submit would put the secret in the same body
// as two non-secret fields, so every future bug in that handler -- logged form values, a proxy
// echoing a query string, an error page that dumps the parsed form -- becomes a credential leak.
// And the audit line for a credential change must not be able to read like an interval change: one
// kind, one action, one place to look.
//
// The current credential is verified through s.authenticator(), the same seam the login path uses,
// so this page cannot hold a stale copy of a password the CLI rotated from another process
// (#1092). The username comes from the operator's session rather than the form: it is not a secret,
// and taking it from the session means the form cannot be pointed at some *other* account to test a
// guess against it.

// The three fields the rotation form posts. They are named here rather than inline because the
// refusal copy, the template and the tests all have to say the same three words, and a typo in one
// of them is a field that silently arrives empty.
const (
	fieldPasswordCurrent = "currentPassword"
	fieldPasswordNext    = "newPassword"
	fieldPasswordConfirm = "confirmPassword"
)

// auditPasswordRotated is written only after a credential was actually replaced.
//
// A refusal writes `auth.failure` -- the same line a wrong login writes -- and never this one. That
// asymmetry is the point: an operator reading the audit trail after an incident needs to tell "the
// password was changed" apart from "somebody tried to change it and could not", and a single kind
// with a success flag in the detail would make the interesting case the one that reads like noise.
// The detail is the username and nothing else. Neither password is written here, to the log, or
// into the re-rendered page, at any point on any path.
const auditPasswordRotated = "admin.password_rotated"

// handlePasswordRotate is POST /settings/password.
func (s *Server) handlePasswordRotate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.sessionFrom(r)
	if !ok {
		// requireAuth already guards this route. The branch is not defence in depth for its own
		// sake: reaching here means the guard was removed or the route was registered without it,
		// and the failure anyone would notice is a rotation that silently accepted an anonymous
		// submit. Redirecting is what every other authenticated page does with nothing to act as.
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// The login throttle, under the same key. A rotation form that skipped it would be an
	// unthrottled oracle for the current password, sitting one click behind a correct session --
	// and the PBKDF2 work is the thing the throttle exists to bound, not merely to hide.
	key := s.throttleKey(r)
	if allowed, wait := s.Throttle.Allow(key); !allowed {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(wait.Seconds())+1))
		s.audit("auth.throttled", key)
		s.passwordRefusal(w, r, http.StatusTooManyRequests,
			fmt.Sprintf("too many failed attempts; try again in %s", wait.Round(time.Second)))
		return
	}

	if err := r.ParseForm(); err != nil || r.ContentLength > maxLoginBody {
		s.passwordRefusal(w, r, http.StatusBadRequest, "the form could not be read")
		return
	}
	current := r.FormValue(fieldPasswordCurrent)
	next := r.FormValue(fieldPasswordNext)
	confirm := r.FormValue(fieldPasswordConfirm)

	// Shape before cryptography: none of these refusals costs a PBKDF2 run, and none of them
	// repeats the login page's sentence, because a mismatched confirmation is not a wrong
	// password and telling the two apart is information the operator needs.
	switch {
	case current == "" || next == "" || confirm == "":
		s.passwordRefusal(w, r, http.StatusUnprocessableEntity,
			"all three password fields need a value")
		return
	case next != confirm:
		s.passwordRefusal(w, r, http.StatusUnprocessableEntity,
			"the two new-password entries do not match")
		return
	case len(next) < auth.MinPasswordLen:
		// The length is reported, the value is not. This is `plock init`'s rule for the same
		// mistake, and the constant is shared rather than copied so the two surfaces cannot
		// disagree about what a valid credential is.
		s.passwordRefusal(w, r, http.StatusUnprocessableEntity,
			fmt.Sprintf("the new password is %d characters; at least %d are required",
				len(next), auth.MinPasswordLen))
		return
	}

	a, err := s.authenticator()
	switch {
	case errors.Is(err, errNoAuthenticator):
		s.audit("auth.no_credential", key)
		s.passwordRefusal(w, r, http.StatusServiceUnavailable,
			"this machine has no administrator account yet")
		return
	case err != nil:
		// Not a wrong password, and not the same sentence as "no account exists": an account does
		// exist and the daemon cannot see it. Answering 401 would send the operator at a credential
		// that was never the problem.
		s.audit("auth.credential_unreadable", key)
		s.passwordRefusal(w, r, http.StatusServiceUnavailable,
			"the administrator credential could not be read")
		return
	}

	ok, err = a.Authenticate(sess.Username, current)
	switch {
	case errors.Is(err, auth.ErrNoCredential):
		s.audit("auth.no_credential", key)
		s.passwordRefusal(w, r, http.StatusServiceUnavailable,
			"this machine has no administrator account yet")
		return
	case err != nil:
		// A verifier that cannot be parsed is the daemon's own broken file. Charging the throttle
		// for it would lock the operator out of the only machine that can fix it, which is the same
		// reasoning the login path uses for the same case.
		s.audit("auth.error", err.Error())
		s.passwordRefusal(w, r, http.StatusInternalServerError, "could not check the credential")
		return
	case !ok:
		// The login page's sentence, the login page's audit kind, and the throttle's failure
		// counter. Nothing here may say "rotation", because this page is reachable by anyone who
		// can reach the login page and a distinct refusal is a way to enumerate what the machine
		// is listening for (#1140).
		s.Throttle.Failure(key)
		s.audit("auth.failure", key)
		s.passwordRefusal(w, r, http.StatusUnauthorized, "wrong username or password")
		return
	}

	// Rotating to the password you just proved you know is not a no-op: the watcher compares
	// verifiers, and a fresh salt makes a re-set of the same password look like a rotation. So the
	// operator would be told the change succeeded, every session including theirs would end within
	// one poll, and nothing about the machine's security would have changed. Refusing is cheaper
	// than explaining that, and it leaks nothing to anyone who has not already passed the current
	// credential. The cost is a second PBKDF2 on a path a person uses once per incident.
	if same, err := a.Authenticate(sess.Username, next); err == nil && same {
		s.passwordRefusal(w, r, http.StatusUnprocessableEntity,
			"the new password is the same as the current one")
		return
	}

	// The verifier is produced before anything is written, so a failure in the KDF leaves the
	// existing credential exactly as it was rather than half-applied (`plock init`'s rule, for the
	// same reason). Dropping the last reference once the KDF has consumed it is not a scrub -- see
	// the longer version of that claim in cmd/plock/init.go.
	verifier, err := auth.Hasher{}.Hash(next)
	next = ""
	confirm = ""
	if err != nil {
		s.audit("auth.error", err.Error())
		s.passwordRefusal(w, r, http.StatusInternalServerError, "the new password could not be stored")
		return
	}

	if err := s.mutateConfig(func(c *config.Config) error {
		c.Admin.Password = verifier
		// Nothing consumes UpdatedAt -- watchCredential compares verifiers on purpose, because a
		// timestamp is a promise every writer has to keep and the write path rewrites the whole
		// document. It is set anyway: it is the field that says when the credential last moved,
		// and a rotation leaving it stale is a lie in a file somebody will read during an incident.
		c.Admin.UpdatedAt = time.Now().UTC()
		return nil
	}); err != nil {
		s.mutateRefused(w, r, "the credential could not be saved", err)
		return
	}

	// Success clears the throttle the login path clears: an operator who typed the current password
	// wrong twice and then got it right should not carry those two attempts into the next incident.
	s.Throttle.Success(key)
	s.audit(auditPasswordRotated, sess.Username)

	cfg, err := s.readConfig()
	if err != nil {
		// The credential is already changed. Saying "the configuration could not be read" now would
		// read as though nothing happened, on a request that did the one thing it promised.
		s.logf("web: %s the admin password was rotated but the settings page could not be re-read: %v",
			requestID(r), err)
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	page := s.settingsPageFor(cfg)
	// The honest sentence about when it takes effect. The watcher polls; it is not a hook. A
	// confirmation that implied the old password stopped working the instant the page redrew would
	// be wrong about the one security property this page exists to provide, and the number is the
	// server's own so the copy cannot drift from the interval the daemon actually runs.
	page.Message = fmt.Sprintf(
		"The admin password was changed. The old one no longer signs in. Every session opened "+
			"under it ends within %d seconds, including this one, so save any half-finished edit "+
			"before you sign in again.", int(s.credentialPoll()/time.Second))
	s.render(w, r, "settings", &page)
}

// passwordRefusal re-renders the settings page naming what has to be fixed.
//
// It renders from the stored document, never from the submitted form, and that is the whole
// criterion: no path through this handler puts either password back on the page. The other two
// fields show what is saved rather than what was typed, which is a deliberate difference from
// POST /settings -- that page re-renders the typed interval because the interval is what the
// operator is holding, while a password box must come back empty whatever happened.
func (s *Server) passwordRefusal(w http.ResponseWriter, r *http.Request, code int, problem string) {
	cfg, err := s.readConfig()
	if err != nil {
		// The form cannot be shown: its other two fields come from the document that just failed to
		// read. Answering in plain text keeps the two things that matter here -- the sentence naming
		// what to fix, and the status this handler chose -- rather than replacing both with a generic
		// "the configuration could not be read", which would send an operator holding a mismatched
		// confirmation at the filesystem.
		s.logf("web: the settings page could not be re-rendered to refuse a rotation: %v", err)
		s.pageRefuse(w, r, code, problem)
		return
	}
	page := s.settingsPageFor(cfg)
	page.Problem = problem
	s.renderStatus(w, r, "settings", &page, code)
}

// settingsPageFor builds the settings view with the two values only the server knows: the length
// floor the browser should ask for, and how long a rotation takes to end a session. Both are read
// from the code that enforces them, so the page cannot promise a rule it does not apply.
func (s *Server) settingsPageFor(cfg *config.Config) settingsPage {
	page := settingsView(cfg)
	page.CredentialPollText = s.credentialPoll().String()
	return page
}

// credentialPoll is the interval the credential watcher actually runs on. Zero means the default,
// the same resolution Start uses; the page states the effective number rather than the constant,
// because a test or an operator who set CredentialPoll would otherwise be shown a sentence about a
// poll that is not happening.
func (s *Server) credentialPoll() time.Duration {
	if s.CredentialPoll > 0 {
		return s.CredentialPoll
	}
	return DefaultCredentialPoll
}
