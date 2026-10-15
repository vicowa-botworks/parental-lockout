package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestJSONLeavesAngleBracketsAlone(t *testing.T) {
	// The reason string is free text an operator typed. HTML-escaping it is not
	// defensive, it is a transformation: the value that comes out of jq is not the value
	// that went in.
	var buf bytes.Buffer
	err := JSON(&buf, struct {
		Reason string `json:"reason"`
	}{Reason: "lock until <17:00> & retry"})
	if err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.Contains(got, `u003c`) || strings.Contains(got, `u0026`) {
		t.Errorf("HTML escaping is on, so a reason does not round trip: %s", got)
	}
	if !strings.Contains(got, "<17:00> & retry") {
		t.Errorf("reason not written verbatim: %s", got)
	}
}

func TestJSONEndsWithExactlyOneNewline(t *testing.T) {
	// `| wc -l` and `| while read` both change behaviour without it, and a second
	// newline makes `| head -1` of a stream of records ambiguous.
	var buf bytes.Buffer
	if err := JSON(&buf, map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(buf.String(), "}\n") || strings.HasSuffix(buf.String(), "\n\n") {
		t.Errorf("output %q does not end in exactly one newline", buf.String())
	}
}

func TestJSONReportsAWriteFailureInsteadOfSwallowingIt(t *testing.T) {
	// A command that cannot write its answer must not exit 0.
	if err := JSON(failWriter{}, map[string]int{"a": 1}); err == nil {
		t.Error("a failing writer reported no error")
	}
}

func TestJSONErrorUsesTheSameGrammarAsASuccess(t *testing.T) {
	var buf bytes.Buffer
	if err := JSONError(&buf, "config_unreadable", errors.New("permission denied")); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{`"error":"config_unreadable"`, `"message":"permission denied"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestReportAlignsLabelsToTheWidestSeenSoFar(t *testing.T) {
	// The reading order is the point: enforcement first, then the detail. Sorting would
	// put "config:" above it.
	r := &Report{}
	r.Field("enforcement", "on").Field("config", "/etc/plock/config.json")
	lines := strings.Split(strings.TrimSuffix(r.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), r.String())
	}
	// "enforcement" is 11 characters and was the widest label when it was written, so it
	// gets one space. "config" arrives second and pads to that same column.
	if lines[0] != "enforcement on" {
		t.Errorf("first field = %q, want %q", lines[0], "enforcement on")
	}
	if want := "config      /etc/plock/config.json"; lines[1] != want {
		t.Errorf("second field = %q, want %q", lines[1], want)
	}
}

func TestReportSectionsSeparateButNeverLeadWithABlankLine(t *testing.T) {
	r := &Report{}
	r.Section("users").Line("kid")
	r.Section("sessions").Line("2")
	got := r.String()
	if strings.HasPrefix(got, "\n") {
		t.Errorf("report starts with a blank line: %q", got)
	}
	if !strings.Contains(got, "kid\n\nsessions") {
		t.Errorf("sections not separated: %q", got)
	}
}

func TestReportWriteReportsAFailure(t *testing.T) {
	r := &Report{}
	r.Field("a", 1)
	if err := r.Write(failWriter{}); err == nil {
		t.Error("a failing writer reported no error")
	}
}

func TestDurationRoundsToTheUnitAnOperatorAsksAbout(t *testing.T) {
	for _, tc := range []struct {
		in   time.Duration
		want string
		why  string
	}{
		{45 * time.Second, "45s", "under a minute is seconds, not a rounded minute"},
		{0, "0s", "a lockout that has ended reads as ended"},
		{-3 * time.Hour, "0s", "a negative wait is never shown as a negative time"},
		{90 * time.Second, "1m30s", "one sub-unit of precision"},
		{2*time.Hour + 5*time.Minute, "2h5m", "hours and minutes"},
		{25 * time.Hour, "1d1h", "a weekend lockout is not 25 hours"},
	} {
		if got := Duration(tc.in); got != tc.want {
			t.Errorf("Duration(%v) = %q, want %q (%s)", tc.in, got, tc.want, tc.why)
		}
	}
}

func TestKeysAreSortedSoTwoRunsProduceTheSameBytes(t *testing.T) {
	// Map iteration is randomized, so an unsorted map-derived output is a format that
	// changes between two runs on the same file.
	m := map[string]int{"zeta": 1, "alpha": 2, "mid": 3}
	first := strings.Join(Keys(m), ",")
	for i := 0; i < 50; i++ {
		if got := strings.Join(Keys(m), ","); got != first {
			t.Fatalf("run %d produced %q, first produced %q", i, got, first)
		}
	}
	if first != "alpha,mid,zeta" {
		t.Errorf("Keys = %q, want sorted", first)
	}
}

func TestExitCodesMatchWhatTheCommandsDocument(t *testing.T) {
	// #11 fixes these three meanings. A fourth value, or a renumbering, breaks every
	// script written against docs/cli.md.
	if OK != 0 || Failure != 1 || Usage != 2 {
		t.Errorf("exit codes are %d/%d/%d, want 0/1/2", OK, Failure, Usage)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("closed pipe") }
