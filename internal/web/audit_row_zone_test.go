package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/audit"
)

// #210: the audit row's stamp must belong to the row, not to whoever reads it.
//
// The trail stores an instant and an offset -- `internal/audit/audit.go:195` writes
// `Format(time.RFC3339Nano)`, so `2026-03-18T21:33:05.000-04:00` -- and no zone name. The page used
// to ask for a name anyway (`{{.At.Format "2006-01-02 15:04:05 MST"}}`), and what `MST` prints for a
// parsed instant depends on the process's `time.Local`: `time.Parse` hands back a value carrying the
// *local* zone when the parsed offset happens to match that zone's abbreviation at that instant, and
// a zone whose name is the bare offset when it does not. Measured, one stored string, three values
// of `TZ` on the same nine lines of Go:
//
//	TZ=America/New_York  MST=2026-03-18 21:33:05 EDT    Z07=2026-03-18 21:33:05-04:00
//	TZ=UTC               MST=2026-03-18 21:33:05 -0400  Z07=2026-03-18 21:33:05-04:00
//	TZ=Asia/Kolkata      MST=2026-03-18 21:33:05 -0400  Z07=2026-03-18 21:33:05-04:00
//
// So the offset is the row's and the abbreviation is the reader's. A daemon started by the service
// manager inherits no `TZ` (`deploy/plock.service` sets none), which means the common host renders
// every row in the trail with a name it invented on the spot -- and the one column a parent quotes
// when they telephone about a lockout is the one column that is a coincidence of the environment.

// oneRowTrail is a trail file as the writer leaves it: an RFC3339Nano instant with a numeric offset
// and no zone name anywhere. It is written to a real file and read by the real `audit.Reader` rather
// than assembled as a `Record`, because the defect lives in the step between those two
// (`internal/audit/reader.go:254`, `time.Parse`) and a hand-built `Record` with `time.UTC` renders
// `UTC` on every host -- a fixture that cannot see the thing it claims to pin.
const oneRowTrail = `{"at":"2026-03-18T21:33:05.000-04:00","kind":"auth.success","username":"parent","actor":"parent"}` + "\n"

// useZone moves the process's local zone for the duration of the test.
//
// `t.Setenv("TZ", …)` alone does not work and this is the second time that has bitten, so it is
// measured rather than remembered: `time.Local` is loaded once at process start, and setting the
// environment afterwards leaves it where it was. Measured on nine lines under TZ=America/New_York --
// after `os.Setenv("TZ","Asia/Kolkata")` the render was still `EDT`; only assigning `time.Local`
// moved it to `-0400`. Assigning it is what a host actually does at startup, so this is the same
// mechanism, driven from inside the test instead of from a shell.
func useZone(t *testing.T, name string) {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("no tzdata for %s on this host (%v); the zones this test compares are exactly what "+
			"it is about, so skipping is honest and failing on a zoneless container is not", name, err)
	}
	before := time.Local
	t.Cleanup(func() { time.Local = before })
	time.Local = loc
}

// whenCell returns the text of the row's `<time>` element and its `datetime` attribute, with the
// whitespace the template's own indentation adds taken out. Both halves are returned because the
// claim being tested is the relationship between them: the attribute has always carried the numeric
// offset, and the human-readable text is what disagreed with it.
func whenCell(t *testing.T, body string) (text, attr string) {
	t.Helper()
	start := strings.Index(body, `<time datetime="`)
	if start < 0 {
		t.Fatalf("the page has no <time> element at all:\n%s", flat(body))
	}
	head := body[start:]
	end := strings.Index(head, "</time>")
	if end < 0 {
		t.Fatalf("an unterminated <time>: %q", head[:min(len(head), 200)])
	}
	open := strings.Index(head, ">")
	// The attribute's closing quote, searched for AFTER the opening one: the first `"` in the
	// string is the one that opens the attribute value, and slicing to it is a range [16:15] panic
	// (measured, on the first run of this helper).
	const open2 = `<time datetime="`
	attrEnd := strings.Index(head[len(open2):], `"`)
	if attrEnd < 0 {
		t.Fatalf("an unterminated datetime attribute: %q", head[:min(len(head), 200)])
	}
	return flat(head[open+1 : end]), head[len(open2) : len(open2)+attrEnd]
}

// The row says the same thing whoever reads it, and says it the way the file says it.
func TestTheAuditRowIsStampedTheSameWhoeverReadsIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte(oneRowTrail), 0o600); err != nil {
		t.Fatal(err)
	}
	want := "2026-03-18 21:33:05 -04:00"
	var seen []string
	for _, zone := range []string{"America/New_York", "UTC", "Asia/Kolkata", "Asia/Kathmandu"} {
		zone := zone
		t.Run(zone, func(t *testing.T) {
			useZone(t, zone)
			s, _ := pageWithSeams(t)
			s.Trail = &audit.Reader{Path: path}
			rec := authGet(t, s, "/audit")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
			}
			text, attr := whenCell(t, rec.Body.String())
			seen = append(seen, zone+": "+text)
			if text != want {
				t.Errorf("the When cell reads %q under TZ=%s; the stored instant is %q, so the cell "+
					"is printing something the trail does not contain", text, zone, want)
			}
			// The attribute is the same instant in ISO form. If the two ever disagree, the page is
			// showing a reader one time and a machine another, and only one of them is in the HTML.
			if attr != "2026-03-18T21:33:05-04:00" {
				t.Errorf("datetime=%q, want the instant the file holds", attr)
			}
		})
	}
	if len(seen) > 1 {
		t.Logf("rendered per zone: %s", strings.Join(seen, " | "))
	}
}

// The mechanism, pinned separately so the fix above cannot be "fixed" by pinning a local zone.
//
// This test asserts the thing that stays true whichever way the template renders: the offset a row
// carries is the row's, the abbreviation it prints is the reader's. A future change that answered
// #210 by forcing `time.Local` to America/New_York on the way in would make the test above pass on
// every host this developer owns, and would still be wrong for a parent whose events were recorded
// across a clock change in a zone the daemon has never heard of.
func TestTheReaderGivesTheOffsetAndNotTheName(t *testing.T) {
	const stored = "2026-03-18T21:33:05.000-04:00"
	for _, zone := range []string{"America/New_York", "UTC", "Asia/Kolkata"} {
		zone := zone
		t.Run(zone, func(t *testing.T) {
			useZone(t, zone)
			at, err := time.Parse(time.RFC3339Nano, stored)
			if err != nil {
				t.Fatal(err)
			}
			if got := at.Format("2006-01-02 15:04:05Z07:00"); got != "2026-03-18 21:33:05-04:00" {
				t.Errorf("the offset moved with the reader: %s", got)
			}
			name := at.Format("MST")
			if zone == "America/New_York" {
				if name != "EDT" {
					t.Errorf("under TZ=America/New_York the abbreviation is %q, want EDT -- if this "+
						"ever reports UTC for a -04:00 instant then the premise of #210 has changed and "+
						"the template decision should be revisited, not this assertion quietly flipped", name)
				}
				return
			}
			if name != "-0400" {
				t.Errorf("under TZ=%s the abbreviation is %q, want the bare offset -0400", zone, name)
			}
		})
	}
}
