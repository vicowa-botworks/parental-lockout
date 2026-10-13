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

// zoneinfoRoot is where the system tz database is looked for. A variable so the
// fallback below can be exercised on a host that does have a database — otherwise
// the branch that runs on slim images is the branch nobody has ever run.
var zoneinfoRoot = "/usr/share/zoneinfo"

func tzZones(t *testing.T) []string {
	t.Helper()
	root := zoneinfoRoot
	if _, err := os.Stat(root); err != nil {
		// No system database. This is not a reason to measure nothing: the package
		// embeds one precisely for hosts like this, so the tests that pin the
		// resolver's bounds are the ones least allowed to skip here. Measuring a
		// curated set keeps the bounds honest; failing to load even those means the
		// embedded database is missing too, and that is a broken build, not a skip.
		var out []string
		for _, z := range boundsZones {
			if _, err := time.LoadLocation(z); err != nil {
				t.Fatalf("no system tz database at %s and the embedded one cannot provide %s (%v): the bounds cannot be measured at all",
					root, z, err)
			}
			out = append(out, z)
		}
		t.Logf("no system tz database at %s: measuring the %d zones the bounds depend on against the embedded database",
			root, len(out))
		return out
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
	// A partial install is worse than none: it looks like a full scan and measures
	// a fraction of the zones, so the bounds come out reassuring rather than wrong.
	//
	// The check has to be on the file, not on LoadLocation. This package imports
	// time/tzdata, so LoadLocation answers from the compiled-in copy even when the
	// file is gone from the tree the scan walks -- the guard would then pass on
	// exactly the install it exists to catch, and the scan below would certify a
	// spread it never measured.
	for _, z := range boundsZones {
		if _, err := os.Stat(filepath.Join(root, z)); err != nil {
			t.Fatalf("%s is missing from the system database at %s: it sets a bound the resolver's fast path rests on, and a scan without it would certify a proof it did not measure",
				z, root)
		}
	}
	return zones
}

// boundsZones are the zones that set the bounds the resolver's shortcuts are
// derived from, or that break them. It is the list to fall back to when there is
// no system database to scan, and the list a partial database must still provide.
//
// Every entry earns its place by being the measurement, not an example:
// the two extremes of offset, the widest clock moves in either direction, every
// zone that has deleted a calendar day, the zones whose clock changes fall at
// midnight, and the narrowest offset state the scan finds.
var boundsZones = []string{
	// The offset extremes: the spread between these two is what uniqueProbe must reach.
	"Pacific/Kiritimati", // +14
	"Pacific/Kanton",     // -12, and a dateline crossing
	// Clock moves wider than an hour, which is what the ±2h guard was wrong about.
	"Antarctica/Casey",  // 3h back every March, 2010-2023
	"Antarctica/Vostok", // 7h in 1994
	"Pacific/Kwajalein", // 23h back in 1969; 24h forward in 1993
	"Pacific/Chatham",   // 91:45 offset, and a half-hour DST step
	"Asia/Kathmandu",    // +05:45
	"Asia/Kolkata",      // +05:30
	// Zones that deleted a calendar date, which is what the three-day walk is for.
	"Pacific/Apia",
	"Pacific/Enderbury",
	"Pacific/Fakaofo",
	// Clock changes at midnight, which is what the noon anchor is for.
	"America/Havana",
	"America/Santiago",
	"Atlantic/Azores",
	// The narrowest offset state the scan finds.
	"America/Boa_Vista",
	// Exactly 2h, the widest change the modern database still makes.
	"Antarctica/Troll",
	// Ordinary zones, so a bound measured only on curiosities is still a bound.
	"Europe/Berlin",
	"America/New_York",
	"Australia/Sydney",
	"Asia/Beirut",
	"Africa/Cairo",
}

// tzZonesAreSystemScan reports whether tzZones walked a real database rather than
// falling back to boundsZones. The "did we cover enough zones" check applies to
// the scan; the fallback is a deliberate, named set and is not measured by count.
func tzZonesAreSystemScan() bool {
	_, err := os.Stat(zoneinfoRoot)
	return err == nil
}

// TestBoundsSurviveWithoutASystemDatabase runs the pinning tests' source of zones
// as a slim image would see it: no /usr/share/zoneinfo at all.
//
// The old helper skipped in exactly this case, which meant the tests that exist to
// catch an unsound fast path went quiet on precisely the hosts the package embeds
// a tz database for. Here the curated set has to carry the bounds on its own, so
// measuring it must still produce the same spread and still find the zones that
// delete a calendar day.
//
// What it cannot show on a host that does have a database is that the embedded copy
// is what answers — Go only reaches for it when the system one fails, and nothing
// in-process can hide the system one. What it does pin is the part that rots
// silently: that the curated list is complete enough to certify the same bound.
func TestBoundsSurviveWithoutASystemDatabase(t *testing.T) {
	saved := zoneinfoRoot
	zoneinfoRoot = "/nonexistent/zoneinfo-for-this-test"
	defer func() { zoneinfoRoot = saved }()

	got := tzZones(t)
	if len(got) != len(boundsZones) {
		t.Fatalf("fallback returned %d zones, want the %d curated ones", len(got), len(boundsZones))
	}
	if tzZonesAreSystemScan() {
		t.Fatal("tzZonesAreSystemScan reports a system scan for a path that does not exist")
	}

	up, down := time.Duration(-100*time.Hour), time.Duration(100*time.Hour)
	for _, z := range got {
		loc, err := time.LoadLocation(z)
		if err != nil {
			t.Fatalf("%s: %v", z, err)
		}
		for h := 0; h < 24*366*40; h += 24 {
			o := offsetAt(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(h)*time.Hour), loc)
			if o > up {
				up = o
			}
			if o < down {
				down = o
			}
		}
	}
	if spread := up - down; spread != 26*time.Hour {
		t.Errorf("the curated zones span %v, want the 26h the full scan measures: uniqueProbe is sized from that spread, so a fallback that measures less would certify less", spread)
	}

	// The dateline discovery has to still work, or the walk test goes quiet too.
	var found []string
	for _, z := range got {
		loc, _ := time.LoadLocation(z)
		if _, ok := deletedDate(loc); ok {
			found = append(found, z)
		}
	}
	if len(found) < 3 {
		t.Errorf("only %d of the curated zones delete a calendar day (%v); the three-day walk would go untested on a slim host", len(found), found)
	}
	t.Logf("fallback: %d zones, spread %v, %d dateline zones", len(got), up-down, len(found))
}

// TestTZDatabaseStaysWithinTheBoundsTheResolverNeeds measures the two bounds the
// resolver's shortcuts are derived from, over every zone and every hour of the
// era a schedule can be asked about.
//
// Both numbers are load-bearing, but they are not the same kind of assertion. The
// offset extremes are asserted at what the scan measures, because the spread
// between them *is* the premise — a tzdata release that adds a +15 zone does not
// make the test over-strict, it makes the fast path unsound, and the test is what
// says so. The state-span floor is asserted well below what is measured (55h
// against 168h), because there the proof only needs a number, and headroom means
// a release that tidies one historical anomaly does not fail the build.
func TestTZDatabaseStaysWithinTheBoundsTheResolverNeeds(t *testing.T) {
	zones := tzZones(t)

	// What the proof consumes. The spread between maxOffset and minOffset has to
	// fit inside uniqueProbe; minStateSpan has to be more than twice it.
	const (
		maxOffset       = 14 * time.Hour  // measured: Pacific/Kiritimati
		minOffset       = -12 * time.Hour // measured: Pacific/Kanton
		maxOffsetZone   = "Pacific/Kiritimati"
		minOffsetZone   = "Pacific/Kanton"
		minStateSpan    = 55 * time.Hour // pinned floor; the scan measures 168h over 1970-2040
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

	if tzZonesAreSystemScan() && covered < minZonesCovered {
		t.Fatalf("only %d zones loaded; the scan would prove nothing", covered)
	}
	t.Logf("%d zones, %d-%d: offsets %v..%v, narrowest offset state %v (%s at %s)",
		covered, scanFrom, scanThrough, widestDown, widestUp,
		narrowest.Round(zoneStepTol), narrowestZone, narrowestWhen.Format(time.RFC3339))

	// The comment above the guards says the extremes are asserted at what the scan
	// measures, and a ceiling alone does not do that: `o > maxOffset` is satisfied
	// just as well by a scan that never reached +14h. So assert both extremes were
	// actually touched.
	//
	// What this catches, measured rather than assumed: the +14h extreme is held by
	// five zones (Kiritimati, Anadyr, Apia, Tongatapu, Etc/GMT-14) and -12h by four
	// (Kanton, Enderbury, Kwajalein, Etc/GMT+12), so deleting one zone file does NOT
	// narrow the measurement -- that case belongs to the file check in tzZones. This
	// is the drift guard: it fires when the pinned constant and the database stop
	// agreeing, which is the same premise failing for the reason the constant exists.
	if widestUp != maxOffset {
		t.Fatalf("the scan never reached offset %v (highest seen %v); %s holds it in the current database. The probe width is derived from the spread between the two extremes, so a narrower measurement is not a passing result: either the scan lost a family of zones, or tzdata moved the extreme and maxOffset has to change with it.",
			maxOffset, widestUp, maxOffsetZone)
	}
	if widestDown != minOffset {
		t.Fatalf("the scan never reached offset %v (lowest seen %v); %s holds it in the current database. The probe width is derived from the spread between the two extremes, so a narrower measurement is not a passing result: either the scan lost a family of zones, or tzdata moved the extreme and minOffset has to change with it.",
			minOffset, widestDown, minOffsetZone)
	}

	// The guard rests on two conditions, and they are not the same inequality.
	//
	// A second instant u reading the same wall clock sits u-t = off(t)-off(u)
	// away, so it is at most `spread` from t. The guard only rules it out if the
	// probe reaches that far: spread <= uniqueProbe. Reaching half of it leaves a
	// second reading outside the window, and the guard then certifies a reading it
	// has not proved unique — which is how the Casey bug survived its own fix.
	//
	// Equal offsets at t±P must also mean nothing changed in between, and that
	// needs the narrowest offset state to be wider than the whole window:
	// minStateSpan > 2*uniqueProbe.
	//
	// Together: spread <= P < minStateSpan/2. Measured, 26h <= 27h < 27.5h. Both
	// bounds are the database's numbers rather than ours, so a tzdata release can
	// close them; the test's job is to shout when that happens, not absorb it.
	if spread := widestUp - widestDown; spread > uniqueProbe {
		t.Fatalf("offset spread %v exceeds uniqueProbe (%v): a second reading of one wall time can sit outside the probe, so the fast path can return a reading that is not unique. Widen uniqueProbe, but it must stay under minStateSpan/2 (%v) — if there is no room left, the fast path goes.",
			spread, uniqueProbe, minStateSpan/2)
	}
	if minStateSpan <= 2*uniqueProbe {
		t.Fatalf("the pinned floor minStateSpan (%v) is not above 2*uniqueProbe (%v): equal offsets at the two probe ends would not mean no change happened between them.",
			minStateSpan, 2*uniqueProbe)
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
			// A jump this wide can swallow local noon, and anchor() stands the
			// whole day-walk on noon existing. Magnitude alone does not pin that:
			// a 24h jump at midnight deletes every hour of a date that no instant
			// ever reads, which the walk survives, while a 13h jump across the
			// middle of a day breaks the anchor for a date that is still real.
			// So ask the anchor about the two days it would actually be called on.
			if jump >= 12*time.Hour {
				for _, probe := range []time.Time{tt.Add(-time.Second), tt} {
					w := probe.In(loc)
					y, mo, dd := w.Date()
					noon := anchor(loc, probe)
					ny, nmo, nd := noon.In(loc).Date()
					nh, nmin, _ := noon.In(loc).Clock()
					if ny != y || nmo != mo || nd != dd || nh != 12 || nmin != 0 {
						t.Fatalf("%s: anchor() on %s (%s) returned %s, not noon on %04d-%02d-%02d — the %v jump at %s deleted that day's noon, and the day-walk anchors there",
							z, probe.Format(time.RFC3339), w.Format(time.RFC3339),
							noon.In(loc).Format(time.RFC3339), y, int(mo), dd, jump,
							tt.Format(time.RFC3339))
					}
				}
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
