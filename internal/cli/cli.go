// Package cli holds the plumbing every plock subcommand needs and none of the policy
// about what any of them means.
//
// It exists because the commands in cmd/plock were about to each re-implement the same
// four things — a JSON writer with stable field names, an exit code, a key/value report
// writer whose errors are actually checked, and a duration that a human can read — and
// the interesting parts of each are the parts a second copy gets subtly wrong. The
// command surface, the flag parsing and the verdicts all stay in cmd/plock, next to the
// commands they belong to; moving them here would relocate the policy away from the tests
// that pin it without making anything reusable.
//
// Nothing in this package knows what a lockout is.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Exit codes. Three, because an operator scripts against the difference between "the
// thing you asked for did not happen" and "you did not ask for it correctly", and a
// single non-zero code forces them to read stderr to find out which.
const (
	OK      = 0
	Failure = 1
	Usage   = 2
)

// JSON writes v with a trailing newline and the field names exactly as the struct tags
// spell them.
//
// HTML escaping is off deliberately. json.Marshal escapes <, > and & by default, so a
// reason string like "lock until <17:00> & retry" arrives on the wire as
// "lock until \u003c17:00\u003e \u0026 retry". Nothing consumes that as HTML — the output
// is for jq and for scripts — and a field whose value changes when it contains an angle
// bracket is a field whose round trip is not the identity, which is the kind of surprise
// a stable machine-readable format is not allowed to have.
//
// The trailing newline is part of the contract rather than decoration: `plock status
// --json | wc -l` and `... | while read` both behave differently without it, and a
// consumer should not have to know which writer produced the bytes.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("writing JSON: %w", err)
	}
	return nil
}

// JSONError writes a failure in the same shape as a success, so a caller that passed
// --json never has to parse two grammars.
//
// The error goes on stderr as JSON, and the human-readable form is still there in the
// "message" field, because the point of --json is that a script can read the failure too
// — "error": null on a broken run is the same lie as a stack trace on a machine-readable
// stream.
func JSONError(w io.Writer, code string, err error) error {
	return JSON(w, struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}{Error: code, Message: err.Error()})
}

// Fail reports an operational failure in whichever grammar the caller asked for and
// returns the exit code to exit with.
//
// With --json the document goes on stderr and stdout stays empty. The two alternatives
// were both worse. On stdout: a caller doing `plock status --json 2>&1 | jq` gets a JSON
// document interleaved with a human line and parses neither. On both streams: that same
// caller gets two documents, and jq reads the first then errors on the second. Empty
// stdout plus exit 1 is the honest pair -- there is no report, and stderr says why in the
// grammar that was asked for.
//
// Every command routes through here so the contract has one implementation rather than
// eight. #11's acceptance criteria name stderr; `status` had been writing to stdout, which
// is exactly how a per-command contract drifts once each command owns its own error path.
//
// The write error on the text path is deliberately not propagated: stderr is the last
// channel there is, and a command that cannot report a failure still has to return a
// non-zero status rather than trade one silence for another.
func Fail(stderr io.Writer, asJSON bool, prefix, code string, err error) int {
	if asJSON {
		if werr := JSONError(stderr, code, err); werr != nil {
			// Failing to write the error is a different fault from the one being
			// reported, and saying so is the only thing that keeps the two apart.
			fmt.Fprintf(stderr, "%s: %v (and writing the JSON error: %v)\n", prefix, err, werr)
		}
		return Failure
	}
	fmt.Fprintf(stderr, "%s: %v\n", prefix, err)
	return Failure
}

// Report accumulates a human-readable report so it can be written once, with the write
// error checked.
//
// This is the same reason doctor.go builds its report into a buffer: a command that
// prints its verdict with a series of unchecked Fprintf calls will print half a verdict
// — or none of it — to a closed pipe and exit 0. For a tool an operator uses to decide
// whether their child's account is locked, "printed nothing" must never be
// indistinguishable from "printed that everything is fine".
type Report struct {
	b     strings.Builder
	width int
}

// Field records one "label: value" line. Fields are emitted in the order they are added,
// not sorted, because a report has a reading order — the answer first, the detail after —
// and sorting it alphabetically puts "config:" above "enforcement:".
func (r *Report) Field(label string, value any) *Report {
	// Aligned to the widest label seen SO FAR, which means an early field can be pushed
	// right by a later, longer one. That is the trade for being able to write a report
	// without knowing its own shape in advance; the alternative is buffering the fields
	// and laying them out at the end, which buys an exact column and costs the ability to
	// add a field from a helper that does not know what else is on the page.
	pad := r.width
	if pad < len(label) {
		pad = len(label)
	}
	fmt.Fprintf(&r.b, "%-*s %v\n", pad, label, value)
	if n := len(label); n > r.width {
		r.width = n
	}
	return r
}

// Section starts a group with a heading and a blank line before it, except at the very
// top where a leading blank line is just a stray.
func (r *Report) Section(heading string) *Report {
	if r.b.Len() > 0 {
		r.b.WriteString("\n")
	}
	fmt.Fprintf(&r.b, "%s\n", heading)
	return r
}

// Line writes one unprefixed line, for rows inside a section.
func (r *Report) Line(format string, args ...any) *Report {
	fmt.Fprintf(&r.b, format, args...)
	if !strings.HasSuffix(format, "\n") {
		r.b.WriteString("\n")
	}
	return r
}

// String returns the report as written.
func (r *Report) String() string { return r.b.String() }

// Write sends the whole report to w and reports any failure.
func (r *Report) Write(w io.Writer) error {
	if _, err := io.WriteString(w, r.b.String()); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	return nil
}

// Duration renders a span the way an operator reads it rather than the way a Go
// constant prints.
//
// time.Duration's own String is precise and unreadable in the way that matters here:
// "1h23m45s" is fine but "2.5h" from arithmetic comes out as "2h30m0s" and a lockout that
// ends in 45 seconds prints as "45s" while one that ends in 45.5 seconds prints as
// "45.5s". For a status line the question is always "how long do I still have to wait",
// and the answer is useful rounded to the largest unit that is not zero, with one
// sub-unit for precision.
func Duration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		// Seconds, because "0s" for a lockout ending now is honest and "1m" is a lie
		// about time that has already passed.
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// Keys returns a map's keys in sorted order, for any output that must be byte-stable
// across runs.
//
// Go randomizes map iteration, so anything built from a map without sorting is a
// machine-readable format that changes between two runs of the same command on the same
// file — which is exactly the property --json exists to provide.
func Keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
