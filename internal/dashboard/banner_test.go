package dashboard

import (
	"strings"
	"testing"
	"time"
)

// banner_test.go tests the sentence, not the fields.
//
// The view model gets every fact right and the banner is the part a parent actually
// reads, so a fact that is correct on the row and wrong in the sentence is still the
// wrong answer the parent acts on. These build the View directly rather than through
// Build because the subject here is how the sentence is composed from rows that are
// already settled.

func TestHeldBannerSaysUntilWhen(t *testing.T) {
	end := time.Date(2026, 10, 15, 6, 30, 0, 0, time.FixedZone("schoolnight", 2*3600))
	v := View{EnforcementOn: true, Rows: []Row{{
		Username: "casey", Enabled: true, LockedOutNow: true,
		RuleName: "school night", EndsAt: end,
	}}}
	got := v.banner()
	if !strings.Contains(got, "until 06:30") {
		t.Errorf("the banner never says how long the hold lasts: %q", got)
	}
	// The other half: the release time must not be dropped in favour of some other
	// clock the sentence could have reached for.
	if strings.Contains(got, "04:30") {
		t.Errorf("the banner rendered the end in a zone other than the profile's: %q", got)
	}
}

// A held row with no end is a different statement, and printing "until 00:00" for it
// invents a release time the daemon never computed.
func TestAHoldWithNoEndDoesNotInventOne(t *testing.T) {
	v := View{EnforcementOn: true, Rows: []Row{{
		Username: "casey", Enabled: true, LockedOutNow: true, RuleName: "override held",
	}}}
	got := v.banner()
	if strings.Contains(got, "until") {
		t.Errorf("the banner announced a release time it does not have: %q", got)
	}
	if !strings.Contains(got, "casey is locked out now") {
		t.Errorf("the banner stopped saying who is held: %q", got)
	}
}

// Three children held and one named is the failure that leaves two of them still
// locked out after the parent fixes the one they were told about.
func TestHeldBannerCountsTheOthers(t *testing.T) {
	v := View{EnforcementOn: true, Rows: []Row{
		{Username: "amy", Enabled: true, LockedOutNow: true},
		{Username: "bo", Enabled: true, LockedOutNow: true},
		{Username: "cy", Enabled: true, LockedOutNow: true},
	}}
	got := v.banner()
	if !strings.Contains(got, "2 other accounts") {
		t.Errorf("the banner named one held account and stayed silent about the rest: %q", got)
	}
}

// One held account must not drag in the plural clause -- "0 other accounts with it"
// is the kind of sentence that makes a parent re-read the whole line.
func TestHeldBannerSaysNothingAboutOthersWhenThereAreNone(t *testing.T) {
	v := View{EnforcementOn: true, Rows: []Row{
		{Username: "amy", Enabled: true, LockedOutNow: true},
	}}
	if got := v.banner(); strings.Contains(got, "other accounts") {
		t.Errorf("the banner counted zero others out loud: %q", got)
	}
}

// A disabled account is not going to be locked out at all, so it has no business
// supplying the time in "locked out again at".
func TestADisabledAccountDoesNotAnnounceALockout(t *testing.T) {
	off := time.Date(2026, 10, 14, 18, 0, 0, 0, time.UTC)
	on := time.Date(2026, 10, 14, 21, 0, 0, 0, time.UTC)
	v := View{EnforcementOn: true, Rows: []Row{
		{Username: "amy", Enabled: false, NextLockAt: off},
		{Username: "bo", Enabled: true, NextLockAt: on},
	}}
	got := v.banner()
	if !strings.Contains(got, "bo is locked out again at 21:00") {
		t.Errorf("the banner quoted the disabled account's window: %q", got)
	}
	if strings.Contains(got, "18:00") {
		t.Errorf("the banner announced a lockout that will never happen: %q", got)
	}
}

// The sentence promises the next thing that happens, so it has to be the earliest one,
// not the first row alphabetically.
func TestTheBannerQuotesTheEarliestWindow(t *testing.T) {
	later := time.Date(2026, 10, 14, 23, 0, 0, 0, time.UTC)
	sooner := time.Date(2026, 10, 14, 19, 30, 0, 0, time.UTC)
	v := View{EnforcementOn: true, Rows: []Row{
		{Username: "amy", Enabled: true, NextLockAt: later},
		{Username: "bo", Enabled: true, NextLockAt: sooner},
	}}
	got := v.banner()
	if !strings.Contains(got, "bo is locked out again at 19:30") {
		t.Errorf("the banner named the first row instead of the next window: %q", got)
	}
}

// The instant carries the zone it was evaluated in, which is the profile's. Rendering
// it in UTC would print a different minute for a child on another zone, and the whole
// point of a per-profile timezone is that the daemon is not that machine.
func TestTheClockIsTheProfilesOwnNotTheDaemons(t *testing.T) {
	zone := time.FixedZone("away", -5*3600)
	at := time.Date(2026, 10, 14, 20, 15, 0, 0, zone)
	if got, want := wallClock(at), "20:15"; got != want {
		t.Errorf("wallClock = %q, want %q -- the same instant is %s in UTC",
			got, want, at.UTC().Format("15:04"))
	}
}

// "There is no next window" has to be a blank. "00:00" is not a blank, it is a wrong
// answer, and it is the one a reader can least easily check.
func TestNoNextWindowIsABlankNotMidnight(t *testing.T) {
	if got := wallClock(time.Time{}); got != "" {
		t.Errorf("wallClock(zero) = %q, want the empty string", got)
	}
}

// A profile this page could not read is not the same fact as a week with nothing in
// it, and the second sentence is the fail-open reading of the first.
func TestAnUnreadableProfileIsNotReportedAsNothingScheduled(t *testing.T) {
	v := View{
		EnforcementOn: true,
		NotEnforcing:  1,
		Rows:          []Row{{Username: "amy", Enabled: true}},
	}
	got := v.banner()
	if !strings.Contains(got, "one account is scheduled to be locked out and is not") {
		t.Errorf("the banner hid the unenforced account: %q", got)
	}
	if strings.Contains(got, "no window is scheduled") {
		t.Errorf("the banner called a broken profile an empty week: %q", got)
	}
}

// The count agrees with the noun, because a banner with "3 accounts is" in it is a
// banner a parent stops reading.
func TestTheUnenforcedClauseAgreesWithItsCount(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{
		{1, "one account is scheduled to be locked out and is not"},
		{3, "3 accounts are scheduled to be locked out and are not"},
	} {
		v := View{EnforcementOn: true, NotEnforcing: tc.n,
			Rows: []Row{{Username: "amy", Enabled: true}}}
		if got := v.banner(); !strings.Contains(got, tc.want) {
			t.Errorf("NotEnforcing=%d banner = %q, want it to contain %q", tc.n, got, tc.want)
		}
	}
}

// The ordering case the earlier sentence used to hide.
//
// Enforcement on; one row scheduled and not enforced (an override, or an unreadable
// profile); a second row with a window later tonight. The upcoming-window branch used to
// return first, so the banner read "Nobody is locked out right now; casey is locked out
// again at 06:30." -- an all-clear about the machine, while a row that is being silently
// unenforced went unmentioned. The row itself shows "open override"; the one sentence a
// parent reads from the next room did not, and this function's own header says the banner
// must not give that all-clear.
func TestAnUpcomingWindowDoesNotVetoTheUnenforcedRow(t *testing.T) {
	zone := time.FixedZone("schoolnight", 2*3600)
	later := time.Date(2026, 10, 15, 20, 0, 0, 0, zone)
	v := View{
		EnforcementOn: true,
		NotEnforcing:  1,
		Rows: []Row{
			{Username: "ada", Enabled: true, OverrideActive: true, OverrideReason: "film night"},
			{Username: "casey", Enabled: true, NextLockAt: later},
		},
	}
	got := v.banner()
	if !strings.Contains(got, "06:30") && !strings.Contains(got, "20:00") {
		t.Errorf("the upcoming window was dropped in favour of the other clause: %q", got)
	}
	if !strings.Contains(got, "one account is scheduled to be locked out and is not") {
		t.Errorf("the unenforced row is missing from the one sentence a parent reads: %q", got)
	}
	// And it must still not read as an all-clear.
	if strings.HasPrefix(got, "Nobody is locked out right now.") {
		t.Errorf("the banner is a bare all-clear: %q", got)
	}
}

// The control: with nothing unenforced, the upcoming-window sentence stays as it was.
// Without this the new clause could be paid for by mentioning an override on every
// ordinary evening.
func TestAnUpcomingWindowAloneStaysAPlainSentence(t *testing.T) {
	zone := time.FixedZone("schoolnight", 2*3600)
	v := View{
		EnforcementOn: true,
		Rows:          []Row{{Username: "casey", Enabled: true, NextLockAt: time.Date(2026, 10, 15, 20, 0, 0, 0, zone)}},
	}
	got := v.banner()
	if strings.Contains(got, "override") || strings.Contains(got, "cannot read") {
		t.Errorf("a machine with nothing wrong is described as though something is: %q", got)
	}
	if !strings.Contains(got, "casey") {
		t.Errorf("the upcoming window lost its subject: %q", got)
	}
}
