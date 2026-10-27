package main

// #168: what doctor does with a session listing that is only partly readable.
//
// doctor was the consumer that got this worst. It read the sessions, then wrote
// `live = nil` over them because the call also returned an error, so an account whose
// session failed to answer was reported as having no session at all — the one wrong answer
// here that reads as good news. The tests below are written so that putting `live = nil`
// back, or dropping the shortfall from the report, fails something.

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// fullCensus is a census with nothing missing. The checks take a census rather than a bare
// slice because the listing can be incomplete, and the verdict has to be able to say which
// of the two it was computed from (#168).
func fullCensus(accounts []unenforcedCandidate) sessionCensus {
	return sessionCensus{People: accounts}
}

func TestCensusKeepsTheRowsItRead(t *testing.T) {
	cfg := &config.Config{
		Version:  config.CurrentVersion,
		Profiles: []config.Profile{{ID: "p1", Name: "nights"}},
		Users:    []config.User{{Username: "casey", ProfileID: "p1", Enabled: true}},
	}
	// Four sessions read, one of five calls failed. The error is the ordinary kind: a
	// show-session that timed out or was refused, not a machine with no loginctl.
	src := &fixtureSessions{
		out: []platform.Session{{
			ID: "c2", Username: "casey", UID: 1002,
			Class: "user", Seat: "seat0", Type: "x11", State: "active",
		}},
		err: errors.New("session c1: loginctl show-session: exit status 1"),
	}

	census := unenforcedCandidates(cfg, src)

	if census.Missing == nil {
		t.Fatal("the shortfall was dropped: the census claims the listing was complete")
	}
	if len(census.People) != 1 {
		t.Fatalf("the rows that read were thrown away: %+v", census.People)
	}
	// The point of keeping them: casey is placed in front of a screen. With `live = nil`
	// restored this is false, and doctor then prints "no session" for a child who is
	// logged in and unlocked.
	if !census.People[0].HasSession {
		t.Errorf("casey reads as having no session; the census is %+v", census.People[0])
	}
}

func TestAnIncompleteCensusIsNeverACleanBillOfHealth(t *testing.T) {
	// The OK sentence is "no account is inside a window with an unlocked screen". Computed
	// over a census that could not read everything, that sentence is about a smaller set of
	// accounts than the one the operator asked about, and it has to say so.
	accounts := []unenforcedCandidate{{Username: "casey", Managed: true, InWindow: true,
		HasSession: true, Lockable: true, Locked: true}}
	got := checkUnenforced(time.Now(), sessionCensus{
		People:  accounts,
		Missing: errors.New("loginctl list-sessions: 1 of 2 session rows could not be read"),
	})

	if got.Status == verdictOK {
		t.Fatalf("an incomplete census returned a clean verdict: %s", got.Detail)
	}
	if !strings.Contains(got.Detail, "could not be read") {
		t.Errorf("the detail does not name the shortfall: %q", got.Detail)
	}
	if !strings.Contains(got.Detail, "casey") && !strings.Contains(got.Detail, "read") {
		t.Errorf("the detail neither lists what was seen nor says it was partial: %q", got.Detail)
	}
	if got.Fix == "" {
		t.Error("an unreadable session has to come with a next step")
	}
}

func TestAnIncompleteCensusRidesOnAFailingVerdict(t *testing.T) {
	// A real finding is still the headline, but the gap must not vanish behind it: "1
	// account is unlocked" over a partial census is a count of what was seen, and the
	// operator reading it as the whole machine is the failure this issue is about.
	accounts := []unenforcedCandidate{{Username: "casey", Managed: true, InWindow: true,
		HasSession: true, Lockable: true, Locked: false}}
	got := checkUnenforced(time.Now(), sessionCensus{
		People:  accounts,
		Missing: errors.New("session c9: bus error"),
	})

	if got.Status != verdictFail {
		t.Fatalf("status = %s, want fail: %s", got.Status, got.Detail)
	}
	if !strings.Contains(got.Detail, "casey") {
		t.Errorf("the finding was displaced by the gap: %q", got.Detail)
	}
	if !strings.Contains(got.Detail, "session c9") {
		t.Errorf("the gap was dropped from a failing verdict: %q", got.Detail)
	}
}

func TestDoctorTryLocksWhatItReadAndSaysWhatItMissed(t *testing.T) {
	// --try is the command an operator runs to answer "will this screen lock". A listing
	// that read three sessions out of four still names the one being asked about, so the
	// read is used and the shortfall is said, rather than the command refusing to answer.
	var out, errOut bytes.Buffer
	chain := &fakeChain{
		names: []string{"logind"},
		sessions: []platform.Session{{
			ID: "c2", Username: "casey", UID: 1002,
			Class: "user", Seat: "seat0", Type: "x11", State: "active",
		}},
		sessionsErr: errors.New("session c1: loginctl show-session: exit status 1"),
	}

	code := runDoctorTry(&out, &errOut, chain, "casey", &report{})

	if code != exitOK {
		t.Fatalf("a partial listing stopped --try: exit %d, stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), `locking "casey"'s session (c2)`) {
		t.Errorf("--try did not lock the session it had read: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "session c1") {
		t.Errorf("the unreadable session went unmentioned: %q", errOut.String())
	}
}

func TestDoctorTryStillRefusesWhenNothingWasRead(t *testing.T) {
	// The other arm: with no rows at all there is nothing to act on, and saying so is the
	// honest answer. Without this the partial-read arm above would be "ignore errors".
	var out, errOut bytes.Buffer
	chain := &fakeChain{
		names:       []string{"logind"},
		sessionsErr: errors.New("loginctl list-sessions: unrecognised output"),
	}

	code := runDoctorTry(&out, &errOut, chain, "casey", &report{})

	if code != exitFailure {
		t.Fatalf("exit = %d, want failure when no session was read at all: %q", code, out.String())
	}
	if !strings.Contains(errOut.String(), "unrecognised output") {
		t.Errorf("the reason was not reported: %q", errOut.String())
	}
}
