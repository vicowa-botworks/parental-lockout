package schedule

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// The resolver's two shortcuts are not guesses about how clocks behave; they are
// consequences of two bounds on the tz database. If a future tzdata release ever
// breaks one, the shortcuts become unsound at the same moment, and this test is
// what says so — before a child stops getting locked out.
//
// It scans every zone the host's database ships. That is the database Go actually
// uses when it is present; the compiled-in copy is the fallback, and it is built
// from the same source.

func tzZones(t *testing.T) []string {
	t.Helper()
	root := "/usr/share/zoneinfo"
	if _, err := os.Stat(root); err != nil {
		t.Skipf("no system tz database at %s", root)
	}
	var zones []string
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, root+"/")
		// Only Region/City style names are zones a config can meaningfully name.
		// The link-annotation and alternate-flavour trees are not, and neither is a
		// bare file at the root (zone.tab, zone1970.tab, iso3166.tab, right/).
		if !strings.Contains(rel, "/") {
			return nil
		}
		if strings.HasPrefix(rel, "posix/") || strings.HasPrefix(rel, "right/") {
			return nil
		}
		zones = append(zones, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	sort.Strings(zones)
	if len(zones) < 100 {
		t.Fatalf("only %d zones under %s; the scan would prove nothing", len(zones), root)
	}
	return zones
}

// TestTZDatabaseStaysWithinTheBoundsTheResolverNeeds measures the two bounds the
// resolver's shortcuts are derived from, over every zone and every hour of the
// era a schedule can be asked about.
//
// Both numbers are load-bearing, and the test asserts the weaker values the proof
// needs rather than the ones measured today, so a tzdata release that nudges
// history does not fail it — only one that breaks the reasoning does.
func TestTZDatabaseStaysWithinTheBoundsTheResolverNeeds(t *testing.T) {
	zones := tzZones(t)

	// What the proof consumes, not what was measured: the measured spread is 26h
	// and the measured minimum state is 96h.
	const (
		maxOffset       = 15 * time.Hour // measured +14h
		minOffset       = -13 * time.Hour
		minStateSpan    = 55 * time.Hour // measured 96h; the guard needs > 2*uniqueProbe
		scanFrom        = 1970
		scanThrough     = 2040
		scanStep        = 6 * time.Hour // finer than minStateSpan, so no state is stepped over
		zoneStepTol     = time.Minute
		minZonesCovered = 400
	)

	covered := 0
	widestUp, widestDown := time.Duration(0), time.Duration(0)
	narrowest := 1000 * 24 * time.Hour
	narrowestZone, narrowestWhen := "", time.Time{}

	for _, z := range zones {
		loc, err := time.LoadLocation(z)
		if err != nil {
			continue
		}
		covered++
		start := time.Date(scanFrom, 1, 1, 0, 0, 0, 0, time.UTC)
		prev := offsetAt(start, loc)
		lastChange := start
		for tt := start.Add(scanStep); tt.Year() <= scanThrough; tt = tt.Add(scanStep) {
			o := offsetAt(tt, loc)
			if o > widestUp {
				widestUp = o
			}
			if o < widestDown {
				widestDown = o
			}
			if o > maxOffset || o < minOffset {
				t.Fatalf("%s: offset %v at %s is outside [%v, %v]; wallInstant's ±%v probe is no longer provable",
					z, o, tt.Format(time.RFC3339), minOffset, maxOffset, uniqueProbe)
			}
			if o != prev {
				if span := tt.Sub(lastChange); span < narrowest {
					narrowest, narrowestZone, narrowestWhen = span, z, tt
				}
				if span := tt.Sub(lastChange); span < minStateSpan {
					t.Fatalf("%s: offset state lasted %v (at %s), below the %v the resolver's sampling assumes",
						z, span, tt.Format(time.RFC3339), minStateSpan)
				}
				prev = o
				lastChange = tt
			}
		}
	}

	if covered < minZonesCovered {
		t.Fatalf("only %d zones loaded; the scan would prove nothing", covered)
	}
	t.Logf("%d zones, %d-%d: offsets %v..%v, narrowest offset state %v (%s at %s)",
		covered, scanFrom, scanThrough, widestDown, widestUp,
		narrowest.Round(zoneStepTol), narrowestZone, narrowestWhen.Format(time.RFC3339))

	// The spread is what bounds how far apart two readings of one wall time can
	// be, and uniqueProbe has to be more than half of it.
	if spread := widestUp - widestDown; spread > 2*uniqueProbe {
		t.Fatalf("offset spread %v exceeds 2*uniqueProbe (%v): the fast path can return a reading that is not unique",
			spread, 2*uniqueProbe)
	}
}

// TestNoZoneDeletesMoreThanOneCalendarDay pins the other half of the lookback.
//
// Evaluate tests the windows that started today, yesterday and the day before.
// Two of those is the whole story on an unbroken calendar; the third is for a
// calendar that is not unbroken. A forward clock move deletes the dates it jumps
// over, and it takes a move of at least 24h to delete one — so as long as no zone
// jumps further than 48h, at most one date can go missing inside a window, and
// three candidate start dates cover every case.
//
// Measured, the biggest forward move anywhere is exactly 24h: five Pacific zones
// crossed the dateline and lost a day.
func TestNoZoneDeletesMoreThanOneCalendarDay(t *testing.T) {
	zones := tzZones(t)

	const maxDeletableJump = 48 * time.Hour

	biggest := time.Duration(0)
	biggestZone, biggestWhen := "", time.Time{}
	for _, z := range zones {
		loc, err := time.LoadLocation(z)
		if err != nil {
			continue
		}
		start := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		prev := offsetAt(start, loc)
		for h := 24; h <= 24*366*150; h += 24 {
			tt := start.Add(time.Duration(h) * time.Hour)
			o := offsetAt(tt, loc)
			if o == prev {
				continue
			}
			jump := o - prev
			if jump > biggest {
				biggest, biggestZone, biggestWhen = jump, z, tt
			}
			if jump >= maxDeletableJump {
				t.Fatalf("%s jumped the clock forward %v at %s: that deletes more than one calendar day, "+
					"and Evaluate only looks back %d of them",
					z, jump, tt.Format(time.RFC3339), windowLookback)
			}
			prev = o
		}
	}

	if windowLookback < 3 {
		t.Fatalf("windowLookback is %d; a zone that deletes a calendar day needs the walk to reach two days back",
			windowLookback)
	}
	t.Logf("biggest forward clock move: %v (%s at %s)", biggest, biggestZone, biggestWhen.Format(time.RFC3339))
}

// TestFastPathNeverDisagreesWithTheExhaustivePath pins the one property that
// makes the fast path safe to keep: it may be faster, it may never be different.
//
// It finds every clock change each zone actually has over fifty years and probes
// the wall times on both sides of each one, in both directions, because that is
// the only place the guard has a chance to be wrong. Probing the current year
// alone finds nothing: the widest clock change anywhere between 2024 and 2046 is
// two hours, so a guard tuned to exactly that passes today and fails the day a
// zone adopts a bigger one — which is what Antarctica/Casey did from 2010 to 2023,
// putting its clocks back three hours every March.
//
// A guard that is merely "usually right" is what let Casey lose the first three
// hours of a Saturday curfew, and no assertion about individual zones would have
// caught a guard that was wrong somewhere else.
func TestFastPathNeverDisagreesWithTheExhaustivePath(t *testing.T) {
	zones := tzZones(t)

	probed, changes := 0, 0
	for _, z := range zones {
		loc, err := time.LoadLocation(z)
		if err != nil {
			continue
		}
		for year := 1970; year <= 2026; year++ {
			start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			prev := offsetAt(start, loc)
			for h := 3; h <= 24*366; h += 3 {
				tt := start.Add(time.Duration(h) * time.Hour)
				o := offsetAt(tt, loc)
				if o == prev {
					continue
				}
				// Bisect back to the change itself, then probe the wall clock on
				// both sides of it and an hour beyond each.
				lo := tt.Add(-3 * time.Hour)
				for tt.Sub(lo) > time.Second {
					mid := lo.Add(tt.Sub(lo) / 2)
					if offsetAt(mid, loc) == offsetAt(lo, loc) {
						lo = mid
					} else {
						tt = mid
					}
				}
				change := tt
				changes++
				before := change.Add(-time.Second).In(loc)
				after := change.In(loc)
				for _, anchor := range []time.Time{before, after} {
					ay, amo, ad := anchor.Date()
					ah, am, _ := anchor.Clock()
					base := ah*60 + am
					for _, off := range []int{-60, 0, 60} {
						min := base + off
						if min < 0 || min > 23*60 {
							continue
						}
						for _, last := range []bool{false, true} {
							got := wallInstant(loc, ay, amo, ad, min, last)
							want := resolveReading(loc,
								time.Date(ay, amo, ad, min/60, min%60, 0, 0, loc), ay, amo, ad, min, last)
							probed++
							if !got.Equal(want) {
								t.Fatalf("%s %04d-%02d-%02d %02d:%02d last=%v: fast path %s, exhaustive %s",
									z, ay, amo, ad, min/60, min%60, last,
									got.UTC().Format(time.RFC3339), want.UTC().Format(time.RFC3339))
							}
						}
					}
				}
				prev = o
			}
		}
	}
	t.Logf("%d clock changes, %d resolutions probed", changes, probed)
}
