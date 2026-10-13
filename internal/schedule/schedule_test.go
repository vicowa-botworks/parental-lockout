package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The tests below use real calendar dates rather than relative times, because a
// lockout that is right "37 hours from now" is not testable — the whole point is
// what happens at 06:30 on a Monday.
//
// Anchors in 2026: 10-10 Sat, 10-11 Sun, 10-12 Mon, 10-14 Wed, 10-15 Thu,
// 10-21 Wed, 10-31 Sat, 11-01 Sun, 12-31 Thu.

func utc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatalf("LoadLocation UTC: %v", err)
	}
	return loc
}

func at(loc *time.Location, y int, mo time.Month, d, hh, mm int) time.Time {
	return time.Date(y, mo, d, hh, mm, 0, 0, loc)
}

func rule(id string, days []string, start, end string) config.Rule {
	return config.Rule{ID: id, Name: id, Days: days, Start: start, End: end, Enabled: true}
}

func profile(tzname string, rules ...config.Rule) *config.Profile {
	return &config.Profile{ID: "p", Name: "test", Timezone: tzname, Rules: rules}
}

var allDays = config.DayNames

func TestEvaluateSameDayWindow(t *testing.T) {
	loc := utc(t)
	// Saturday 2026-10-10, 20:00-21:00, no wrap.
	p := profile("UTC", rule("bedtime", []string{"sat"}, "20:00", "21:00"))

	cases := []struct {
		name   string
		now    time.Time
		active bool
	}{
		{"an hour before", at(loc, 2026, 10, 10, 19, 0), false},
		{"one minute before", at(loc, 2026, 10, 10, 19, 59), false},
		{"exactly at start", at(loc, 2026, 10, 10, 20, 0), true},
		{"one minute in", at(loc, 2026, 10, 10, 20, 1), true},
		{"last minute", at(loc, 2026, 10, 10, 20, 59), true},
		{"exactly at end", at(loc, 2026, 10, 10, 21, 0), false},
		{"a minute after", at(loc, 2026, 10, 10, 21, 1), false},
		{"same time on a weekday", at(loc, 2026, 10, 12, 20, 30), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Evaluate(p, tc.now)
			if v.Active != tc.active {
				t.Fatalf("Active = %v at %s, want %v", v.Active, tc.now.Format(time.RFC3339), tc.active)
			}
			if !tc.active {
				if v.Rule != nil || !v.StartedAt.IsZero() || !v.EndsAt.IsZero() {
					t.Errorf("inactive verdict still names a window: %+v", v)
				}
				return
			}
			if v.Rule == nil || v.Rule.ID != "bedtime" {
				t.Fatalf("Rule = %+v, want bedtime", v.Rule)
			}
			if want := at(loc, 2026, 10, 10, 20, 0); !v.StartedAt.Equal(want) {
				t.Errorf("StartedAt = %s, want %s", v.StartedAt, want)
			}
			if want := at(loc, 2026, 10, 10, 21, 0); !v.EndsAt.Equal(want) {
				t.Errorf("EndsAt = %s, want %s", v.EndsAt, want)
			}
		})
	}
}

// A window that crosses midnight belongs to the day it starts on. Sunday
// 21:00-06:30 covers Sunday evening into Monday morning, and Monday evening is
// not covered at all.
func TestEvaluateOvernightWrap(t *testing.T) {
	loc := utc(t)
	p := profile("UTC", rule("schoolnight", []string{"sun"}, "21:00", "06:30"))

	cases := []struct {
		name   string
		now    time.Time
		active bool
	}{
		{"sunday one minute before", at(loc, 2026, 10, 11, 20, 59), false},
		{"sunday at start", at(loc, 2026, 10, 11, 21, 0), true},
		{"sunday 23:59", at(loc, 2026, 10, 11, 23, 59), true},
		{"monday 00:00", at(loc, 2026, 10, 12, 0, 0), true},
		{"monday 03:00", at(loc, 2026, 10, 12, 3, 0), true},
		{"monday 06:29", at(loc, 2026, 10, 12, 6, 29), true},
		{"monday 06:30 at end", at(loc, 2026, 10, 12, 6, 30), false},
		{"monday 06:31", at(loc, 2026, 10, 12, 6, 31), false},
		{"monday evening is not a start day", at(loc, 2026, 10, 12, 22, 0), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Evaluate(p, tc.now)
			if v.Active != tc.active {
				t.Fatalf("Active = %v at %s, want %v", v.Active, tc.now.Format(time.RFC3339), tc.active)
			}
			if !tc.active {
				return
			}
			// The window is the Sunday one even at Monday 03:00, and the rule
			// that says "sun" is the one responsible.
			if v.Rule == nil || v.Rule.ID != "schoolnight" {
				t.Fatalf("Rule = %+v, want schoolnight", v.Rule)
			}
			if want := at(loc, 2026, 10, 11, 21, 0); !v.StartedAt.Equal(want) {
				t.Errorf("StartedAt = %s, want the Sunday start %s", v.StartedAt, want)
			}
			if want := at(loc, 2026, 10, 12, 6, 30); !v.EndsAt.Equal(want) {
				t.Errorf("EndsAt = %s, want the Monday end %s", v.EndsAt, want)
			}
		})
	}
}

// A window that ends where it starts is empty, not a 24-hour lockout. Validate
// rejects the shape, but Load keeps a document it cannot validate, so the
// evaluator has to hold the line too.
func TestEvaluateZeroLengthWindowIsNeverActive(t *testing.T) {
	loc := utc(t)
	p := profile("UTC", rule("typo", allDays, "00:00", "00:00"))

	for h := 0; h < 24; h++ {
		for _, m := range []int{0, 30, 59} {
			now := at(loc, 2026, 10, 12, h, m)
			if v := Evaluate(p, now); v.Active {
				t.Fatalf("00:00-00:00 was active at %s: %+v", now.Format(time.RFC3339), v)
			}
		}
	}
	if v := Evaluate(p, at(loc, 2026, 10, 12, 12, 0)); !v.NextStart.IsZero() {
		t.Errorf("NextStart = %s, want zero for a profile that can never open a window", v.NextStart)
	}
}

// 00:00-23:59 is what a parent is told to write for an all-day rule. It is 1439
// minutes: the last minute of the day is free. Tracked in #16.
func TestEvaluateAllDayWindowIsOneMinuteShort(t *testing.T) {
	loc := utc(t)
	p := profile("UTC", rule("allday", allDays, "00:00", "23:59"))

	cases := []struct {
		name   string
		now    time.Time
		active bool
	}{
		{"midnight", at(loc, 2026, 10, 12, 0, 0), true},
		{"midday", at(loc, 2026, 10, 12, 12, 0), true},
		{"23:58", at(loc, 2026, 10, 12, 23, 58), true},
		{"23:59, the gap", at(loc, 2026, 10, 12, 23, 59), false},
		{"23:59:59, still the gap", at(loc, 2026, 10, 12, 23, 59).Add(59 * time.Second), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if v := Evaluate(p, tc.now); v.Active != tc.active {
				t.Fatalf("Active = %v at %s, want %v", v.Active, tc.now.Format(time.RFC3339), tc.active)
			}
		})
	}
}

func TestEvaluateMonthAndYearBoundaries(t *testing.T) {
	loc := utc(t)

	t.Run("october to november", func(t *testing.T) {
		// Saturday 31 Oct 23:00 into Sunday 1 November.
		p := profile("UTC", rule("weekend", []string{"sat"}, "23:00", "06:00"))

		v := Evaluate(p, at(loc, 2026, 11, 1, 3, 0))
		if !v.Active {
			t.Fatalf("not active at 03:00 on 1 November: %+v", v)
		}
		if want := at(loc, 2026, 11, 1, 6, 0); !v.EndsAt.Equal(want) {
			t.Errorf("EndsAt = %s, want %s", v.EndsAt, want)
		}
	})

	t.Run("december to january", func(t *testing.T) {
		// Thursday 31 December 2026 23:00 into Friday 1 January 2027.
		p := profile("UTC", rule("newyear", []string{"thu"}, "23:00", "06:00"))

		v := Evaluate(p, at(loc, 2027, 1, 1, 3, 0))
		if !v.Active {
			t.Fatalf("not active at 03:00 on 1 January 2027: %+v", v)
		}
		if want := at(loc, 2027, 1, 1, 6, 0); !v.EndsAt.Equal(want) {
			t.Errorf("EndsAt = %s, want %s", v.EndsAt, want)
		}
		if v.Rule == nil || v.Rule.ID != "newyear" {
			t.Errorf("Rule = %+v, want newyear", v.Rule)
		}
	})

	t.Run("february to march in a leap year", func(t *testing.T) {
		// 2028-02-29 is a Tuesday.
		p := profile("UTC", rule("leap", []string{"tue"}, "23:00", "06:00"))

		v := Evaluate(p, at(loc, 2028, 3, 1, 3, 0))
		if !v.Active {
			t.Fatalf("not active at 03:00 on 1 March 2028: %+v", v)
		}
		if want := at(loc, 2028, 3, 1, 6, 0); !v.EndsAt.Equal(want) {
			t.Errorf("EndsAt = %s, want %s", v.EndsAt, want)
		}
	})
}

// The account stays locked while any rule is active, so EndsAt is the latest end
// among the rules holding it and Rule names the one that ends last.
func TestEvaluateOverlappingRules(t *testing.T) {
	loc := utc(t)
	monday := at(loc, 2026, 10, 12, 21, 30)

	t.Run("the later end wins", func(t *testing.T) {
		p := profile("UTC",
			rule("earlier", []string{"mon"}, "20:00", "22:00"),
			rule("later", []string{"mon"}, "21:00", "23:00"),
		)

		v := Evaluate(p, monday)
		if !v.Active {
			t.Fatal("not active with two rules covering 21:30")
		}
		if v.Rule == nil || v.Rule.ID != "later" {
			t.Errorf("Rule = %+v, want later — it is the one that ends last", v.Rule)
		}
		if want := at(loc, 2026, 10, 12, 23, 0); !v.EndsAt.Equal(want) {
			t.Errorf("EndsAt = %s, want %s", v.EndsAt, want)
		}
		// StartedAt is the responsible rule's own opening, not the earliest of
		// the two: the verdict describes one window, and the daemon re-locks on
		// the strength of EndsAt.
		if want := at(loc, 2026, 10, 12, 21, 0); !v.StartedAt.Equal(want) {
			t.Errorf("StartedAt = %s, want %s", v.StartedAt, want)
		}
	})

	t.Run("profile order does not decide it", func(t *testing.T) {
		p := profile("UTC",
			rule("later", []string{"mon"}, "21:00", "23:00"),
			rule("earlier", []string{"mon"}, "20:00", "22:00"),
		)

		v := Evaluate(p, monday)
		if v.Rule == nil || v.Rule.ID != "later" {
			t.Errorf("Rule = %+v, want later even though it comes first", v.Rule)
		}
	})

	t.Run("a tie names the first rule in the profile", func(t *testing.T) {
		p := profile("UTC",
			rule("first", []string{"mon"}, "20:00", "22:00"),
			rule("second", []string{"mon"}, "21:00", "22:00"),
		)

		v := Evaluate(p, monday)
		if v.Rule == nil || v.Rule.ID != "first" {
			t.Errorf("Rule = %+v, want first — the tie must resolve deterministically", v.Rule)
		}
	})

	t.Run("an overnight rule and a same-day rule", func(t *testing.T) {
		// Sunday 21:00-06:30 and Monday 05:00-08:00 both cover Monday 06:00.
		p := profile("UTC",
			rule("schoolnight", []string{"sun"}, "21:00", "06:30"),
			rule("morning", []string{"mon"}, "05:00", "08:00"),
		)

		v := Evaluate(p, at(loc, 2026, 10, 12, 6, 0))
		if !v.Active {
			t.Fatal("not active at Monday 06:00")
		}
		if v.Rule == nil || v.Rule.ID != "morning" {
			t.Errorf("Rule = %+v, want morning (ends 08:00, later than 6, 30)", v.Rule)
		}
		if want := at(loc, 2026, 10, 12, 8, 0); !v.EndsAt.Equal(want) {
			t.Errorf("EndsAt = %s, want %s", v.EndsAt, want)
		}

		// Past 06:30 the overnight rule has let go; only the morning one holds.
		v = Evaluate(p, at(loc, 2026, 10, 12, 7, 0))
		if v.Rule == nil || v.Rule.ID != "morning" {
			t.Errorf("Rule = %+v, want morning", v.Rule)
		}
	})

	t.Run("a gap between two rules releases the account", func(t *testing.T) {
		p := profile("UTC",
			rule("first", []string{"mon"}, "20:00", "21:00"),
			rule("second", []string{"mon"}, "22:00", "23:00"),
		)

		if v := Evaluate(p, at(loc, 2026, 10, 12, 21, 30)); v.Active {
			t.Errorf("active in the gap between two rules: %+v", v)
		}
	})
}

func TestEvaluateNextStart(t *testing.T) {
	loc := utc(t)
	p := profile("UTC", rule("wednesday", []string{"wed"}, "20:00", "06:00"))

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "from a thursday, next wednesday",
			now:  at(loc, 2026, 10, 15, 12, 0),
			want: at(loc, 2026, 10, 21, 20, 0),
		},
		{
			name: "from the wednesday itself, later that day",
			now:  at(loc, 2026, 10, 14, 12, 0),
			want: at(loc, 2026, 10, 14, 20, 0),
		},
		{
			name: "from inside the window, the following wednesday",
			now:  at(loc, 2026, 10, 14, 22, 0),
			want: at(loc, 2026, 10, 21, 20, 0),
		},
		{
			name: "from the monday after, four days ahead",
			now:  at(loc, 2026, 10, 19, 9, 0),
			want: at(loc, 2026, 10, 21, 20, 0),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Evaluate(p, tc.now)
			if !v.NextStart.Equal(tc.want) {
				t.Errorf("NextStart = %s, want %s", v.NextStart.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
		})
	}

	t.Run("the earliest of several rules", func(t *testing.T) {
		p := profile("UTC",
			rule("late", []string{"fri"}, "20:00", "22:00"),
			rule("soon", []string{"tue"}, "09:00", "10:00"),
		)

		v := Evaluate(p, at(loc, 2026, 10, 12, 12, 0)) // Monday
		if want := at(loc, 2026, 10, 13, 9, 0); !v.NextStart.Equal(want) {
			t.Errorf("NextStart = %s, want %s", v.NextStart, want)
		}
	})

	// NextStart is the next opening, not the next lockout after this one ends: a
	// second rule can open while the account is already held. Documented on the
	// field, and pinned here so the wording cannot quietly change.
	t.Run("can fall before EndsAt while active", func(t *testing.T) {
		p := profile("UTC",
			rule("long", []string{"mon"}, "20:00", "23:00"),
			rule("extra", []string{"mon"}, "22:00", "22:30"),
		)

		v := Evaluate(p, at(loc, 2026, 10, 12, 21, 0))
		if !v.Active {
			t.Fatal("not active at Monday 21:00")
		}
		if !v.NextStart.Equal(at(loc, 2026, 10, 12, 22, 0)) {
			t.Errorf("NextStart = %s, want 22:00", v.NextStart)
		}
		if !v.NextStart.Before(v.EndsAt) {
			t.Errorf("expected NextStart (%s) before EndsAt (%s)", v.NextStart, v.EndsAt)
		}
	})

	// Eight days covers every weekday plus one, so a weekly rule is always
	// reachable — including the case where today is its day and today's window
	// has already closed. That is what the extra day buys, and it is the
	// difference between a dashboard that says "next Monday" and one that says
	// nothing at all.
	t.Run("a weekly rule whose day has passed is found next week", func(t *testing.T) {
		p := profile("UTC", rule("oneday", []string{"mon"}, "09:00", "10:00"))

		v := Evaluate(p, at(loc, 2026, 10, 12, 12, 0)) // Monday afternoon
		if want := at(loc, 2026, 10, 19, 9, 0); !v.NextStart.Equal(want) {
			t.Errorf("NextStart = %s, want %s", v.NextStart, want)
		}
	})

	t.Run("zero when no rule can ever open", func(t *testing.T) {
		p := profile("UTC", rule("typo", allDays, "00:00", "00:00"))

		v := Evaluate(p, at(loc, 2026, 10, 12, 12, 0))
		if !v.NextStart.IsZero() {
			t.Errorf("NextStart = %s, want zero", v.NextStart)
		}
	})
}

func TestEvaluateEmptyAndNilProfiles(t *testing.T) {
	loc := utc(t)
	now := at(loc, 2026, 10, 12, 12, 0)

	t.Run("nil profile", func(t *testing.T) {
		var v Verdict
		if got := Evaluate(nil, now); got != v {
			t.Errorf("Evaluate(nil) = %+v, want the zero Verdict", got)
		}
	})

	t.Run("no rules", func(t *testing.T) {
		if got := Evaluate(profile("UTC"), now); got != (Verdict{}) {
			t.Errorf("Evaluate(empty) = %+v, want the zero Verdict", got)
		}
	})

	t.Run("all rules disabled", func(t *testing.T) {
		r := rule("off", allDays, "00:00", "23:59")
		r.Enabled = false

		if got := Evaluate(profile("UTC", r), now); got != (Verdict{}) {
			t.Errorf("Evaluate(disabled) = %+v, want the zero Verdict", got)
		}
	})

	t.Run("a disabled rule does not stop its neighbour", func(t *testing.T) {
		off := rule("off", allDays, "00:00", "23:59")
		off.Enabled = false

		p := profile("UTC", off, rule("on", allDays, "11:00", "12:00"))
		v := Evaluate(p, at(loc, 2026, 10, 12, 11, 30))
		if !v.Active || v.Rule == nil || v.Rule.ID != "on" {
			t.Errorf("verdict = %+v, want the enabled rule to hold", v)
		}
	})
}

// A malformed rule is skipped and reported, never a panic and never a silent
// lockout. This is the hand-edit path: Load keeps a document Validate would
// reject, so the evaluator is the last thing standing between a typo and a child
// who is locked out at the wrong minute.
func TestEvaluateSkipsMalformedRules(t *testing.T) {
	loc := utc(t)
	now := at(loc, 2026, 10, 12, 12, 0)

	cases := []struct {
		name    string
		rule    config.Rule
		wantErr string
	}{
		{"start not a time", rule("bad1", []string{"mon"}, "9:00", "14:00"), `start "9:00"`},
		{"end not a time", rule("bad2", []string{"mon"}, "11:00", "24:00"), `end "24:00"`},
		{"empty start", rule("bad3", []string{"mon"}, "", "14:00"), `start ""`},
		{"unknown day", rule("bad4", []string{"monnday"}, "11:00", "14:00"), `day "monnday"`},
		{"no days", rule("bad5", nil, "11:00", "14:00"), "no days"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := profile("UTC", tc.rule)

			if v := Evaluate(p, now); v.Active {
				t.Errorf("a malformed rule locked the account out: %+v", v)
			}
			errs := ProfileErrors(p)
			if len(errs) != 1 {
				t.Fatalf("ProfileErrors = %v, want one problem", errs)
			}
			if !strings.Contains(errs[0].Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to mention %q", errs[0], tc.wantErr)
			}
			if !strings.Contains(errs[0].Error(), tc.rule.ID) {
				t.Errorf("error = %q, want it to name the rule %q", errs[0], tc.rule.ID)
			}
		})
	}

	t.Run("a good rule beside a bad one still enforces", func(t *testing.T) {
		p := profile("UTC",
			rule("bad", []string{"mon"}, "25:00", "26:00"),
			rule("good", allDays, "11:00", "13:00"),
		)

		v := Evaluate(p, now)
		if !v.Active || v.Rule.ID != "good" {
			t.Errorf("verdict = %+v, want the good rule to hold the account", v)
		}
		if errs := ProfileErrors(p); len(errs) != 1 {
			t.Errorf("ProfileErrors = %v, want only the bad rule reported", errs)
		}
	})

	t.Run("a rule with no id is still identified", func(t *testing.T) {
		r := rule("", []string{"mon"}, "99:99", "10:00")

		errs := ProfileErrors(profile("UTC", r))
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "rules[0]") {
			t.Errorf("ProfileErrors = %v, want the problem located at rules[0]", errs)
		}
	})

	t.Run("a disabled malformed rule is not reported", func(t *testing.T) {
		r := rule("off", []string{"monday!"}, "99:99", "99:99")
		r.Enabled = false

		if errs := ProfileErrors(profile("UTC", r)); errs != nil {
			t.Errorf("ProfileErrors = %v, want nothing: a rule that is off is not silently failing", errs)
		}
	})

	t.Run("nil profile", func(t *testing.T) {
		if errs := ProfileErrors(nil); errs != nil {
			t.Errorf("ProfileErrors(nil) = %v, want nil", errs)
		}
	})
}

func TestLocation(t *testing.T) {
	if got := Location("Europe/Berlin"); got.String() != "Europe/Berlin" {
		t.Errorf("Location(Europe/Berlin) = %v", got)
	}
	if got := Location(""); got != time.Local {
		t.Errorf("Location(empty) = %v, want time.Local", got)
	}
	if got := Location("Mars/Olympus_Mons"); got != time.Local {
		t.Errorf("Location(unknown) = %v, want time.Local", got)
	}
}

// The evaluator reads the day names from config, so the two cannot disagree
// about which names exist.
func TestWeekdayMatchesConfigDayNames(t *testing.T) {
	want := []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday, time.Thursday,
		time.Friday, time.Saturday, time.Sunday,
	}

	for i, name := range config.DayNames {
		got, ok := weekday(name)
		if !ok {
			t.Fatalf("weekday(%q) not found, but config.DayNames lists it", name)
		}
		if got != want[i] {
			t.Errorf("weekday(%q) = %v, want %v", name, got, want[i])
		}
	}

	if _, ok := weekday("Mon"); ok {
		t.Error("weekday accepted an upper-case name; the documented names are lower case")
	}
	if _, ok := weekday("monday"); ok {
		t.Error("weekday accepted a long name; the documented names are three letters")
	}
}

// config and schedule have to mean the same thing by a rule, or a rule can save
// cleanly and then enforce nothing. Both packages read the clock through
// config.ParseClock for exactly that reason; these pin the two places the
// meanings could still drift apart.
//
// They matter because of how a bad document gets here: Validate refuses it, but
// Load keeps it so that one typo cannot switch the whole daemon off. So the
// evaluator is the last thing that has to know what a rule means.
func TestPackagesAgreeOnWhatARuleMeans(t *testing.T) {
	t.Run("a time Validate accepts is a time the evaluator parses", func(t *testing.T) {
		cases := []struct {
			clock  string
			parses bool
		}{
			{"00:00", true},
			{"23:59", true},
			{"09:00", true},  // this is the zero-padded form; "9:00" is not
			{"24:00", false}, // midnight is 00:00, and 24:00 is not a time of day
			{"9:00", false},
			{"12:60", false},
			{"12:5", false},
			{"", false},
			{"01:00 ", false}, // the format is anchored at both ends
			{" 01:00", false},
		}

		for _, tc := range cases {
			c := config.Default()
			c.Admin.Username = "parent"
			c.Admin.Password = "verifier"
			c.Profiles = []config.Profile{{
				ID: "p", Name: "p", Timezone: "UTC",
				// End is fixed away from every value in the table, so the only problem
				// Validate can raise here is the format of Start itself.
				Rules: []config.Rule{{ID: "r", Days: []string{"mon"}, Start: tc.clock, End: "18:00", Enabled: true}},
			}}

			_, got := config.ParseClock(tc.clock)
			if got != tc.parses {
				t.Errorf("ParseClock(%q) = %v, want %v", tc.clock, got, tc.parses)
			}

			accepted := c.Validate() == nil
			if accepted != tc.parses {
				t.Errorf("Validate accepts %q = %v but ParseClock parses it = %v; the two must not disagree",
					tc.clock, accepted, got)
			}
		}
	})

	// The degenerate window is the shape where "empty" and "all day" are both
	// defensible readings, and the packages have to pick the same one. Validate
	// tells the parent it enforces nothing; the evaluator must agree, or that
	// message is a lie and a parent approving the typo gets the opposite of what
	// they were told.
	t.Run("the window Validate calls empty is the window that never fires", func(t *testing.T) {
		loc := utc(t)
		r := rule("typo", allDays, "08:00", "08:00")

		c := config.Default()
		c.Admin.Username = "parent"
		c.Admin.Password = "verifier"
		c.Profiles = []config.Profile{{ID: "p", Name: "p", Timezone: "UTC", Rules: []config.Rule{r}}}

		err := c.Validate()
		if err == nil {
			t.Fatal("Validate accepted the window the evaluator treats as empty")
		}
		if !strings.Contains(err.Error(), "enforces nothing") {
			t.Fatalf("Validate does not say the window enforces nothing: %v", err)
		}

		p := profile("UTC", r)
		for h := 0; h < 24; h++ {
			if v := Evaluate(p, at(loc, 2026, 10, 12, h, 0)); v.Active {
				t.Fatalf("the window Validate calls empty was active at %d:00", h)
			}
		}
	})
}
