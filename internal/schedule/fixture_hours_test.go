package schedule

// The pages fixture is a photograph, and a photograph is only evidence if it is the same photograph
// tomorrow (#207). Its dashboard row is decided by `Evaluate`, so an enabled rule whose window ends
// later than the all-day rule's end moves that row at some hours of the day and not others -- which
// is exactly what `r-curfew`'s `06:30` did for the three hours 21:00 <= t < 24:00, refusing the
// capture for a quarter of every day and publishing none of the set (#220). D-0017 moved the end to
// `00:00`, where it releases at the same instant the all-day rule does and the tie names the first
// rule in the profile (TestEvaluateOverlappingRules, schedule_test.go:294).
//
// This is the claim measured instead of argued, over the fixture's own bytes: `pages-setup.sh` is
// parsed rather than copied, so a rule added there is in this sweep the moment it is committed and
// this file cannot drift from the document it describes. cmd/plock/docs_pinned_test.go reads the
// README for the same reason -- a check that retypes its input is a second copy, not a guard.
//
// The BEFORE arm is not decoration. Rewriting the one value D-0017 changed reproduces #220's table
// exactly -- two verdicts, the second one for 180 minutes -- which is what proves the sweep can see a
// verdict that moves. A sweep that reports one verdict whether or not the fixture holds would pass on
// the defect it exists to catch.

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// schoolProfileFromFixture reads the `school` profile out of the capture fixture. The heredoc's one
// interpolated value is the listen port, which is outside the `profiles` array, so the array is
// valid JSON as written.
func schoolProfileFromFixture(t *testing.T, endForCurfew string) *config.Profile {
	t.Helper()
	raw, err := os.ReadFile("../../testing/capture/pages-setup.sh")
	if err != nil {
		t.Fatalf("the fixture is the input to this check: %v", err)
	}
	src := string(raw)
	if endForCurfew != "" {
		// D-0017's own value, so the BEFORE arm is one string away from the committed document.
		re := regexp.MustCompile(`"start": "21:00", "end": "00:00"`)
		if !re.MatchString(src) {
			t.Fatal(`the fixture no longer holds start 21:00 / end 00:00 on r-curfew, so the BEFORE ` +
				`arm has nothing to rewrite: this test and testing/capture/pages-setup.sh have come ` +
				`apart, and the hour-independence claim in DECISIONS.md D-0017 is no longer measured`)
		}
		src = re.ReplaceAllString(src, `"start": "21:00", "end": "`+endForCurfew+`"`)
	}
	m := regexp.MustCompile(`(?s)"profiles": (\[.*?\]),\n\s*"users"`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal(`no "profiles" array was found in testing/capture/pages-setup.sh -- the fixture is ` +
			`written in a shape this check can no longer read, and the check is the only thing that ` +
			`would notice a second enabled rule re-opening #220`)
	}
	var profiles []*config.Profile
	if err := json.Unmarshal([]byte(m[1]), &profiles); err != nil {
		t.Fatalf("the fixture's profiles array is not the JSON this package reads: %v", err)
	}
	for _, p := range profiles {
		if p.ID == "school" {
			return p
		}
	}
	t.Fatal("the fixture has no `school` profile, so the dashboard row this sweep follows is gone")
	return nil
}

// wantVerdict is the pair `pages.py` declares for the casey row in `Page("/", ...).state`:
// `locked out until 00:00` and the rule `Every day, every hour`. It is repeated here rather than
// parsed out of the Python for one reason: a gate that reads its expectation out of the same file it
// is guarding cannot fail when that file is edited to match whatever the page happened to render.
const wantVerdict = "00:00 / Every day, every hour"

// verdictAnswer is the pair the dashboard's casey row renders: the clock in the `until` cell and the
// rule in the `Why` cell. Both, because #220 moved the second one too -- a check on the clock alone
// would have called the frame unchanged while the row quietly re-blamed itself at 21:00.
//
// The clock only, never the date: `dashboard.tmpl` prints `until 15:04`, so a verdict that says
// "tomorrow 00:00" on every date renders identically on every date. Keying this on the date instead
// reported 365 distinct verdicts for a fixture whose row does not change -- measured on the first run
// of this test, and worth the sentence because the wrong key would have failed a correct fixture and
// taught whoever read it that #220 was still open.
func verdictAnswer(v Verdict) string {
	if !v.Active {
		return "inactive"
	}
	name := "(no rule)"
	if v.Rule != nil {
		name = v.Rule.Name
	}
	return v.EndsAt.Format("15:04") + " / " + name
}

func TestThePagesFixtureHoldsOneDashboardVerdictForAWWholeDay(t *testing.T) {
	profile := schoolProfileFromFixture(t, "")
	loc, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		t.Fatalf("the fixture names a zone this host cannot read: %v", err)
	}
	// The fixture's own declared day (`fixture.sh`, the `CAPTURE_STAMP` default: 21:34:05 at -04:00),
	// walked from its first minute so the sweep is the calendar day and not a 24-hour span that starts
	// at noon. The noon anchor belongs to the evaluator's own day-walk (#1747), not to the caller.
	day := time.Date(2026, 3, 18, 0, 0, 0, 0, loc)

	t.Run("every minute of the declared day", func(t *testing.T) {
		seen := map[string][]string{}
		for m := 0; m < 24*60; m++ {
			now := day.Add(time.Duration(m) * time.Minute)
			a := verdictAnswer(Evaluate(profile, now))
			seen[a] = append(seen[a], now.In(loc).Format("15:04"))
		}
		if len(seen) != 1 {
			for a, mins := range seen {
				t.Errorf("verdict %q holds %d of the 1440 minutes (first %s, last %s)",
					a, len(mins), mins[0], mins[len(mins)-1])
			}
			t.Errorf("%d verdicts on one day: the fixture photographs a different dashboard row at "+
				"another hour, which is #220 arriving again. `pages.py` will refuse the frame for the "+
				"hours it disagrees with `Page(\"/\").state`, and publish none of the set.", len(seen))
		}
		for a := range seen {
			if a != wantVerdict {
				t.Errorf("the fixture's casey row reads %q for the whole day, and the driver's "+
					"declared state (`Page(\"/\").state` in testing/capture/pages.py) says %q. One of "+
					"the two is wrong; a photograph is only evidence if the row it shows is the row "+
					"the driver promises.", a, wantVerdict)
			}
			t.Logf("one verdict for all 1440 minutes: %s", a)
		}
	})

	t.Run("every day of the year at the hours a rule can disagree", func(t *testing.T) {
		// 20:59 the curfew has not opened, 21:00 it has, 21:01 both are open with the all-day rule
		// still ahead, 23:59 the last minute before the all-day rule's own end. Those are the hours
		// on which any window that crosses midnight can overtake the all-day rule.
		seen := map[string]string{}
		for d := 0; d < 366; d++ {
			for _, hm := range [][2]int{{20, 59}, {21, 0}, {21, 1}, {23, 59}} {
				now := time.Date(2026, 1, 1, hm[0], hm[1], 0, 0, loc).
					AddDate(0, 0, d).In(loc)
				a := verdictAnswer(Evaluate(profile, now))
				if _, ok := seen[a]; !ok {
					seen[a] = now.Format("2006-01-02 15:04")
				}
			}
		}
		if len(seen) != 1 {
			for a, at := range seen {
				t.Errorf("verdict %q first seen at %s", a, at)
			}
			t.Errorf("%d distinct verdicts across 2026: the artifact would be a function of the date "+
				"it was taken on, not only the hour (#207 cause 5, #220)", len(seen))
		}
		// Uniformly wrong is still wrong. A fixture whose row said `06:30` all year would satisfy a
		// check that only asks whether the answer moves, and `pages.py` would refuse the frame at
		// capture time while this gate stayed green and said nothing to anyone.
		if at, ok := seen[wantVerdict]; !ok {
			for a, at := range seen {
				t.Errorf("no day of 2026 rendered %q; every one of the 1464 instants read %q, first "+
					"at %s", wantVerdict, a, at)
			}
		} else {
			t.Logf("%d instants across 2026, all %s (first %s)", 366*4, wantVerdict, at)
		}
	})
}

// TestThePagesFixtureVerdictSweepCanSeeASecondEnd is the discrimination. Without it the sweep above
// is a program that prints one line and passes, and would have passed on #220.
func TestThePagesFixtureVerdictSweepCanSeeASecondEnd(t *testing.T) {
	profile := schoolProfileFromFixture(t, "06:30")
	loc, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 3, 18, 0, 0, 0, 0, loc)
	seen := map[string]int{}
	for m := 0; m < 24*60; m++ {
		seen[verdictAnswer(Evaluate(profile, day.Add(time.Duration(m)*time.Minute)))]++
	}
	if len(seen) != 2 {
		t.Fatalf("rewriting the fixture's curfew end to 06:30 gave %d verdicts, want 2 -- #220's three "+
			"hours are no longer reproducible here, so the day sweep above proves nothing about a "+
			"verdict that moves: %v", len(seen), seen)
	}
	// The two verdicts #220 measured, with their widths: the curfew owns the row for exactly the 180
	// minutes 21:00..23:59, and the all-day rule for the other 1260. A threshold at 1 would let a
	// sweep that saw the defect at one minute a year pass as faithfully as one that saw it for three
	// hours, and the three hours are the measurement the issue is built on.
	if got := seen["06:30 / Weeknight curfew including the weekend"]; got != 180 {
		t.Fatalf("the 06:30 verdict held %d minutes, want 180 (21:00..23:59, the window #220 measured): %v",
			got, seen)
	}
	if got := seen["00:00 / Every day, every hour"]; got != 1260 {
		t.Fatalf("the all-day verdict held %d minutes, want 1260: %v", got, seen)
	}
}
