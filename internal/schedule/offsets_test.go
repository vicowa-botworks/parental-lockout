package schedule

import (
	"testing"
	"time"
)

// offsetsAround is a grid, and a grid can step over the thing it is looking for.
//
// The exhaustive resolver builds its candidates from the offsets this returns, so
// an offset that is missing here is a reading of the written time that never gets
// considered — and a window whose true start is that reading resolves to the wrong
// instant. Nothing in the resolver's own logic would notice: it searches the
// candidates it was given and returns the best of them.
//
// So the requirement is stated directly: every offset in effect within the reach
// of a repeated reading has to be in the list. The reach is the same 26h the fast
// path's proof uses — two instants reading the same wall clock are at most that far
// apart — and the grid is 6h against the 55h minimum offset state the bounds test
// pins, which is what
// makes "cannot be missed" arithmetic rather than optimism.
func TestOffsetsAroundFindsEveryOffsetThatCouldMatter(t *testing.T) {
	zones := tzZones(t)

	const reach = 26 * time.Hour

	probed := 0
	for _, z := range zones {
		loc, err := time.LoadLocation(z)
		if err != nil {
			continue
		}
		// Probe at the changes themselves. Sampling ordinary days finds an offset
		// list that is trivially complete — every instant within reach shares one
		// offset — so a grid coarse enough to step over a real transition passes
		// them all. The first version of this test did exactly that, and a grid
		// halved in density survived it.
		start := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		prev := offsetAt(start, loc)
		for h := 24; h <= 24*366*70; h += 24 {
			tt := start.Add(time.Duration(h) * time.Hour)
			o := offsetAt(tt, loc)
			if o == prev {
				continue
			}
			// Bisect to the change, then stand on both sides of it and at the far
			// reach of a repeated reading.
			lo := tt.Add(-24 * time.Hour)
			for tt.Sub(lo) > time.Second {
				mid := lo.Add(tt.Sub(lo) / 2)
				if offsetAt(mid, loc) == offsetAt(lo, loc) {
					lo = mid
				} else {
					tt = mid
				}
			}
			prev = o

			for _, shift := range []time.Duration{-reach, -90 * time.Minute, -time.Second,
				0, time.Second, 90 * time.Minute, reach} {
				t0 := tt.Add(shift)
				have := offsetsAround(t0, loc)
				for d := -27 * time.Hour; d <= 27*time.Hour; d += time.Hour {
					want := offsetAt(t0.Add(d), loc)
					probed++
					found := false
					for _, x := range have {
						if x == want {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("%s at %s: offsetsAround returned %v but %s is in effect with offset %v",
							z, t0.Format(time.RFC3339), have,
							t0.Add(d).Format(time.RFC3339), want)
					}
				}
			}
		}
	}
	t.Logf("%d offset lookups around every change, none missed", probed)
}
