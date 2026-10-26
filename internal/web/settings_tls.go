package web

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The transport form: `tls`, `certFile`, `keyFile` — the third and last slice of #99.
//
// The reason this form exists at all is the reason it is the most careful of the three. The
// other two settings pages write values the running daemon re-reads on its next tick, so a
// wrong value is wrong until it is fixed. A transport value is different: `web.New` resolves
// the transport once, at start-up, and returns an error if it cannot. So a form that accepted
// `custom` with a certificate that is not there would hand the operator a document the *next
// daemon start refuses to boot on* — and the daemon is the thing serving the page that would
// have been their way out. #99 names this exactly: "a tls value that names a certificate that
// does not exist must be refused before it is saved".
//
// That is why the refusal below calls `LoadCustom` — the same function `web.New` calls — rather
// than a lighter check of its own. A second, weaker copy of the start-up rule is how a form
// ends up accepting something the daemon then refuses, which is the defect this page exists to
// prevent; the poll-interval field on the form above makes the same argument about
// `config.MinPollSeconds`.
//
// It is also why the read behind `LoadCustom` had to stop being unbounded (see
// `readCertFile` in tls.go): a form that hands a path to `tls.LoadX509KeyPair` is a form that
// can point the daemon at `/dev/zero` and watch it allocate until the runtime kills it, or at
// a named pipe and watch it park in `open` forever. Both were measured on this box before the
// bound went in, and both are pinned by tests here rather than asserted in a comment.

// The three form field names. They match the JSON keys (`settings.tls`, `settings.certFile`,
// `settings.keyFile`) for the reason `settingsFieldPoll` gives: the sentence naming a field is
// then the string the operator would grep for in the config file, and the two surfaces cannot
// end up calling one setting two things.
const (
	settingsFieldTLS  = "tls"
	settingsFieldCert = "certFile"
	settingsFieldKey  = "keyFile"
)

// tlsPairRefusal is what the form says when the daemon could not load the pair it was given.
//
// One sentence for every reason, and the reason is not in it. `LoadCustom` distinguishes a
// missing file from an unreadable one from a key that does not match its certificate, and this
// handler runs as root — so a response that said "no such file or directory" for one path and
// "permission denied" for another would let a signed-in operator map the root filesystem one
// request at a time, and a third sentence ("failed to find any PEM data") would say whether a
// file starts with a PEM block. The same reasoning makes a failed login refuse without naming
// what was wrong (#1140).
//
// The trade only holds if the reason survives somewhere a root-only reader goes, so the handler
// writes it to the daemon's log with the request id next to it, the way `pageError` routes a
// suppressed detail. This sentence used to send the operator to `plock doctor` instead, and
// doctor cannot keep that promise: its TLS check is handed `cfg.Settings.CertFile` and nothing
// else — `checkTLS` takes one path — so a document holding A's certificate and B's key, which
// is the reason most likely to arrive from this form, reports `tls | ok`. A foreground start is
// no better: it can only report a pair that reached the document *without* this form, the state
// the form exists to prevent. So the sentence names the one place that actually has the reason.
const tlsPairRefusal = "the certificate and key you named could not be loaded, so the daemon " +
	"would not start with them. Nothing was changed. The reason is in the daemon's log, on the " +
	"line that carries this request's id, and nowhere else: `plock doctor` reads the certificate " +
	"alone, so a key that is missing, unreadable, or not the pair to that certificate appears " +
	"only in the log."

// handleSettingsTLSSave is POST /settings/tls.
//
// A route of its own rather than three more fields on POST /settings, for the reason the
// password form gives: the forms answer different questions and a refusal on one has no
// business re-rendering the other's fields as though they had been submitted. It also keeps a
// transport change out of the same parsed form as the enforcement switch, so a truncated body
// cannot half-apply the two.
func (s *Server) handleSettingsTLSSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.pageRefuse(w, r, http.StatusBadRequest, "the form could not be read")
		return
	}
	cfg, err := s.readConfig()
	if err != nil {
		s.pageError(w, r, "the configuration could not be read", err)
		return
	}

	// Trimmed, because a path with a stray space in it is the mistake this form is most likely
	// to receive from a browser (a double-click selects a path without its trailing space, and a
	// paste often adds one). Trimming is not correcting: a value that is refused is refused as
	// typed, and nothing here rewrites a path the operator did not write.
	typedTLS := strings.TrimSpace(r.FormValue(settingsFieldTLS))
	typedCert := strings.TrimSpace(r.FormValue(settingsFieldCert))
	typedKey := strings.TrimSpace(r.FormValue(settingsFieldKey))
	refuse := func(problem string) {
		s.renderTLSRefusal(w, r, cfg, problem, typedTLS, typedCert, typedKey)
	}

	mode, refusal := tlsModeWord(typedTLS)
	if refusal != "" {
		refuse(refusal)
		return
	}

	// The two paths are required by `custom` and optional otherwise, and the emptiness rule is
	// config's (`validate.go`: custom with an empty certFile or keyFile is refused), restated
	// here only because the document check can say "certFile or keyFile is empty" while the
	// operator holding this form needs to know which box. Validate still runs inside mutateConfig
	// on the document about to be saved, so what follows is a better sentence, not a weaker gate.
	if mode == TLSCustom && (typedCert == "" || typedKey == "") {
		// The sentence names the box the operator has to fill, so it has to agree with what it
		// names: one box "is required", both of them "are required". Sharing missingPathFields
		// with the both-empty case is what used to print "settings.certFile and settings.keyFile
		// is required".
		verb, name := " is", "Name the file; "
		if typedCert == "" && typedKey == "" {
			verb, name = " are", "Name both files; "
		}
		refuse("settings." + missingPathFields(typedCert, typedKey) + verb +
			` required when tls is "custom": the daemon would start with nothing to serve the ` +
			"handshake with. " + name + "nothing was changed.")
		return
	}

	// The load check runs only for `custom`, because `custom` is the only mode that reads these
	// paths. Under `auto` or `off` they are inert, and refusing a path that is merely unused
	// would make the ordinary order of operations impossible — write the config first, issue the
	// certificate second, is a legitimate way to provision a machine, and nothing about it
	// produces a daemon that fails to start.
	if mode == TLSCustom {
		// The page asks for absolute paths, and this is where that sentence is made true. A
		// relative path resolves against the daemon's working directory, which is not the
		// directory anyone is typing from: a relative pair that happens to load under the unit's
		// WorkingDirectory is a daemon that comes back refusing to serve a page after somebody
		// edits the unit file. config.Validate takes either, because the document's job is to
		// describe the machine rather than second-guess it, so this gate is not weaker than the
		// document's — it is the one the form's own words promised.
		if rel := relativePathFields(typedCert, typedKey); rel != "" {
			refuse("settings." + rel + " must be an absolute path: the daemon resolves a relative " +
				"one against its own working directory, not the one you are typing from, and a " +
				"certificate found that way is a start-up failure waiting for someone to edit the " +
				"unit file. Nothing was changed.")
			return
		}
		if _, err := LoadCustom(typedCert, typedKey, s.now); err != nil {
			// The generic sentence goes to the browser and the reason comes here, with the
			// request id next to it — the same trade `pageError` makes, and it is the half this
			// handler used to forget. Without it the page tells a signed-in operator to go read
			// a log that contains nothing but `POST /settings/tls 422`, and the disclosure
			// argument above is a promise with nothing behind it. The two paths are safe to
			// write down: the success path already puts them, quoted, in the audit trail.
			s.logf("web: %s settings.tls refused %s + %s: %v", requestID(r),
				strconv.Quote(typedCert), strconv.Quote(typedKey), err)
			refuse(tlsPairRefusal)
			return
		}
	}

	var changes []string
	if err := s.mutateConfig(func(c *config.Config) error {
		if c.Settings.TLS != mode {
			changes = append(changes, fmt.Sprintf("tls %s to %s", strconv.Quote(c.Settings.TLS), mode))
			c.Settings.TLS = mode
		}
		if c.Settings.CertFile != typedCert {
			changes = append(changes, fmt.Sprintf("certFile %s to %s",
				strconv.Quote(c.Settings.CertFile), strconv.Quote(typedCert)))
			c.Settings.CertFile = typedCert
		}
		if c.Settings.KeyFile != typedKey {
			changes = append(changes, fmt.Sprintf("keyFile %s to %s",
				strconv.Quote(c.Settings.KeyFile), strconv.Quote(typedKey)))
			c.Settings.KeyFile = typedKey
		}
		if len(changes) == 0 {
			return errNothingChanged
		}
		return nil
	}); err != nil {
		if errors.Is(err, errNothingChanged) {
			page := s.settingsPageFor(cfg)
			page.TLSMode, page.CertField, page.KeyField = mode, typedCert, typedKey
			page.Message = "Nothing changed, so nothing was written."
			s.render(w, r, "settings", &page)
			return
		}
		s.mutateRefused(w, r, "the transport settings could not be saved", err)
		return
	}

	// Same kind as the switch and the interval, because it is the same fact: an operator changed
	// a setting, and the trail has to say which one and who. The field names are in the detail
	// rather than in a kind of their own so that "what changed on this machine" is one grep.
	s.audit("settings.changed", fmt.Sprintf("%s by %s", strings.Join(changes, "; "),
		sessionName(s, r)))

	page := s.settingsPageFor(cfg)
	page.TLSMode, page.CertField, page.KeyField = mode, typedCert, typedKey
	page.Message = tlsConfirmation(changes, mode)
	// Rendered, not redirected, for the reason handleSettingsSave gives: the page they land on is
	// the page they submitted from, and the sentence is the acceptance criterion. Re-posting is
	// idempotent — the second submit finds nothing changed and writes nothing.
	s.render(w, r, "settings", &page)
}

// tlsModeWord reads the transport radio trio.
//
// An absent field is refused rather than defaulted, which is the enforcement switch's argument
// arriving at a second control: the page always submits one of the three, so a request carrying
// none is a truncated body or a hand-built POST, and the reading this project refuses to make
// silently is the one that guesses (#873). Guessing `auto` here would be the mild version of
// the same mistake as guessing `off` on the switch.
func tlsModeWord(raw string) (mode string, refusal string) {
	switch raw {
	case TLSAuto, TLSOff, TLSCustom:
		return raw, ""
	case "":
		return "", "settings.tls was not submitted, so the daemon cannot tell which transport you " +
			"meant. Pick one of " + tlsModeList() + " explicitly; nothing was changed."
	default:
		return "", "settings.tls must be one of " + tlsModeList() + ", not " + strconv.Quote(raw) +
			"; nothing was changed."
	}
}

// tlsModeList is the accepted set, spelled from the constants the handler actually acts on, so
// the sentence cannot advertise a mode this file forgot to implement.
func tlsModeList() string {
	return strings.Join([]string{TLSAuto, TLSOff, TLSCustom}, ", ")
}

// missingPathFields names the box that is empty, which is the one thing the document-level
// sentence cannot say because it is written to cover either one being missing.
func missingPathFields(cert, key string) string {
	switch {
	case cert == "" && key == "":
		return settingsFieldCert + " and settings." + settingsFieldKey
	case cert == "":
		return settingsFieldCert
	default:
		return settingsFieldKey
	}
}

// relativePathFields names the boxes holding a path that is not absolute, for the refusal that
// makes the form's "absolute paths" sentence true. Same shape as missingPathFields, and the same
// reason for it: the document-level check can cover either, while the person holding this form
// needs to know which box.
func relativePathFields(cert, key string) string {
	switch {
	case !filepath.IsAbs(cert) && !filepath.IsAbs(key):
		return settingsFieldCert + " and settings." + settingsFieldKey
	case !filepath.IsAbs(cert):
		return settingsFieldCert
	case !filepath.IsAbs(key):
		return settingsFieldKey
	default:
		return ""
	}
}

// tlsConfirmation words the sentence under the form after an accepted change.
//
// The restart clause is on every confirmation because it is true of every transport change and
// it is the one thing an operator would otherwise assume the wrong way: the browser is already
// on a connection, so a page that says "Saved: tls off" reads like the wire changed just now.
// It did not. `web.New` chose the transport when the daemon started and nothing re-reads it.
func tlsConfirmation(changes []string, mode string) string {
	var b strings.Builder
	b.WriteString("Saved: ")
	b.WriteString(strings.Join(changes, "; "))
	b.WriteString(". The transport is chosen when the daemon starts, so this takes effect the next ")
	b.WriteString("time it starts — the page you are reading is still on the transport it came up with.")
	switch mode {
	case TLSOff:
		b.WriteString(" With TLS off the admin password and the session cookie cross the network in ")
		b.WriteString("clear text, which is what `plock doctor` warns about on every run.")
	case TLSCustom:
		b.WriteString(" The pair you named loaded just now, which is the same check the daemon makes ")
		b.WriteString("when it starts; if it is moved or expires, the next start fails and says so.")
	case TLSAuto:
		b.WriteString(" auto mints a self-signed certificate under the state directory at startup and ")
		b.WriteString("renews it, so there is no path to keep correct.")
	}
	return b.String()
}

// renderTLSRefusal re-renders the settings page with the transport boxes holding what was typed.
//
// Only the three transport fields are echoed. The interval box keeps showing the stored number,
// because this submit never carried it and a box that displayed something the daemon is not
// running would be a second lie on a page that is already refusing.
func (s *Server) renderTLSRefusal(w http.ResponseWriter, r *http.Request, cfg *config.Config,
	problem, typedTLS, typedCert, typedKey string) {
	page := s.settingsPageFor(cfg)
	page.TLSMode = typedTLS
	page.CertField = typedCert
	page.KeyField = typedKey
	page.Problem = problem
	s.renderStatus(w, r, "settings", &page, http.StatusUnprocessableEntity)
}
