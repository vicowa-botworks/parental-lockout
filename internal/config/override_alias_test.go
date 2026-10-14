package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// slot names one list in a document so its length and capacity can be compared.
type slot struct {
	name             string
	length, capacity int
}

// kind groups a slot by which clip statement is responsible for it, so the fixture can
// be required to exercise each one.
func (s slot) kind() string {
	switch {
	case s.name == "Profiles", s.name == "Users", s.name == "Overrides",
		s.name == "Settings.LockBackends":
		return s.name
	case strings.HasSuffix(s.name, ".Rules"):
		return "Rules"
	case strings.HasSuffix(s.name, ".Days"):
		return "Days"
	}
	return "other:" + s.name
}

// Load must hand back no slice with room to append into, at any depth.
//
// This is the structural defence against the aliasing class this file is about, and
// it is asserted here rather than left to the edit helpers' discipline: encoding/json
// grows slices by doubling, so a decoded document routinely carries spare capacity,
// and two holders that each append then write the same spare slot. With len == cap
// every append reallocates on its own and the shape cannot occur.
//
// The nested lists are the point of the loop. A clip that covers the three top-level
// slices and forgets Profile.Rules and Rule.Days leaves those exactly as appendable as
// before, while reading as though the class were closed. That is not a hypothetical
// here: the first version of this test used the package's shared sample(), whose
// profiles hold two rules each -- and two elements is exactly what the decoder
// allocates without spare, so the Rules clip was present, unexercised, and green. The
// fixture below is sized so every clip site has something to clip, and the precondition
// check insists on that per site rather than in aggregate.
func TestLoadHandsBackNoSpareCapacity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := wideSample().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// slotsOf collects every list a document carries, by name, with its length and
	// capacity. Written out here rather than obtained from the package: a helper that
	// walked the config through the code under test would inherit exactly the slices
	// the clip failed to reach, and a missed list would hide its own miss.
	slotsOf := func(c *Config) []slot {
		var out []slot
		add := func(name string, length, capacity int) {
			out = append(out, slot{name, length, capacity})
		}
		add("Profiles", len(c.Profiles), cap(c.Profiles))
		add("Users", len(c.Users), cap(c.Users))
		add("Overrides", len(c.Overrides), cap(c.Overrides))
		add("Settings.LockBackends", len(c.Settings.LockBackends), cap(c.Settings.LockBackends))
		for i, p := range c.Profiles {
			add(fmt.Sprintf("Profiles[%d].Rules", i), len(p.Rules), cap(p.Rules))
			for j, r := range p.Rules {
				add(fmt.Sprintf("Profiles[%d].Rules[%d].Days", i, j), len(r.Days), cap(r.Days))
			}
		}
		return out
	}

	// The precondition, measured WITHOUT Load. Asking Load whether decoded documents
	// carry spare capacity can only ever answer "no" once Load clips them, which turns
	// the guard into a test that cancels itself while looking like a pass. So the same
	// bytes are decoded directly here: if the raw decoder leaves no spare capacity
	// either, the clip has nothing to do and this test pins nothing.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var undecorated Config
	if err := json.Unmarshal(raw, &undecorated); err != nil {
		t.Fatalf("raw decode: %v", err)
	}
	before := slotsOf(&undecorated)
	// Per site, not in aggregate. An aggregate "something was loose" is satisfied by
	// one lucky list, which is exactly how the Rules clip shipped unexercised once:
	// every clip site must have had something to clip, or that site is code no test
	// has ever run.
	looseByKind := map[string]int{}
	total := 0
	for _, c := range before {
		if c.length < c.capacity {
			total++
			looseByKind[c.kind()]++
		}
	}
	for _, want := range []string{
		"Profiles", "Users", "Overrides", "Settings.LockBackends", "Rules", "Days",
	} {
		if looseByKind[want] == 0 {
			t.Errorf("no %s list had spare capacity before the clip ran, so the clip on %s is never exercised and a bug there would go undetected (fixture too small: %d of %d lists were loose overall)",
				want, want, total, len(before))
		}
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, c := range slotsOf(cfg) {
		if c.capacity != c.length {
			t.Errorf("%s: Load returned len %d but cap %d — appending to it would write into capacity a sibling holder may still use (%d of %d lists were loose before the clip ran)",
				c.name, c.length, c.capacity, total, len(before))
		}
	}
}

// Two holders of one array both grant an unlock.
//
// The sharing is built explicitly here rather than borrowed from encoding/json's
// growth policy. Load now clips spare capacity away (see
// TestLoadHandsBackNoSpareCapacity), which is the fix; this test is what keeps the
// edit helpers' own copy-on-write rule pinned after that, because a Config assembled
// by hand — or by a future constructor that does not go through Load — can still carry
// room to append into. It also survives a Go release that ever returns
// exact-capacity slices, which would otherwise make this regression untestable.
//
// The direction is what makes it worth a test of its own: the grant that vanishes
// fails closed and reads as a bug someone reports, while the grant that appears from
// nowhere unlocks a child nobody unlocked.
func TestAddOverrideDoesNotWriteIntoAnArrayAnotherConfigHolds(t *testing.T) {
	base := time.Date(2026, 10, 12, 20, 40, 0, 0, time.UTC)
	existing := []Override{
		{Username: "one", Until: base, Reason: "existing"},
		{Username: "two", Until: base, Reason: "existing"},
		{Username: "three", Until: base, Reason: "existing"},
	}

	// sharedWithRoom is one array, len 3 cap 4, that two Configs will both hold.
	sharedWithRoom := func() *Config {
		c := sample()
		room := make([]Override, len(existing), len(existing)+1)
		copy(room, existing)
		c.Overrides = room
		return c
	}

	t.Run("fresh grant for two different children", func(t *testing.T) {
		cfg := sharedWithRoom()
		// The precondition, asserted rather than assumed: without spare capacity the
		// append allocates on its own and the test proves nothing.
		if cap(cfg.Overrides) == len(cfg.Overrides) {
			t.Fatalf("the shared array has len==cap==%d; without spare capacity this test cannot fail", cap(cfg.Overrides))
		}

		a, b := *cfg, *cfg
		now := base.Add(24 * time.Hour)

		a.AddOverride(Override{Username: "alice", Until: now.Add(time.Hour), Reason: "GRANT-A"})
		b.AddOverride(Override{Username: "bob", Until: now.Add(time.Hour), Reason: "GRANT-B"})

		if _, ok := a.ActiveOverride("alice", now); !ok {
			t.Errorf("alice's grant is gone from the config that granted it: b's append wrote over the slot a had just filled")
		}
		if _, ok := a.ActiveOverride("bob", now); ok {
			t.Errorf("bob holds an unlock in a config that never granted him one — AddOverride appended into the array a still shares")
		}
		if _, ok := b.ActiveOverride("bob", now); !ok {
			t.Errorf("bob's own config lost his grant")
		}

		// Surviving a round trip is what lifts this above a stale in-memory view: Save
		// copies whatever the shared array now holds, so the wrong document is what the
		// enforcement loop reads after a restart.
		path := filepath.Join(t.TempDir(), "config.json")
		if err := a.Save(path); err != nil {
			t.Fatalf("Save(a): %v", err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatalf("re-Load: %v", err)
		}
		if _, ok := back.ActiveOverride("alice", now); !ok {
			t.Errorf("alice's grant did not survive the save that a made")
		}
		for _, o := range back.Overrides {
			if o.Username == "bob" {
				t.Errorf("the saved document carries bob's ungranted unlock (%q) — the corruption reached disk", o.Reason)
			}
		}
	})

	// The amend path is the one that masked the original bug rather than showing it.
	// AddOverride removes any existing grant first, and cut allocates a fresh array, so
	// the append that follows is safe — for the wrong reason. Nothing here would fail if
	// a refactor moved the removal after the append, or dropped the copy and let cut's
	// allocation be the accidental protection again. Two holders, both amending the same
	// child, is the shape that pins that ordering-dependent safety down.
	t.Run("amending a grant both holders already have", func(t *testing.T) {
		amended := make([]Override, len(existing), len(existing)+2)
		copy(amended, existing)
		amended = append(amended, Override{Username: "alice", Until: base.Add(time.Hour), Reason: "ORIGINAL"})
		cfg := sample()
		cfg.Overrides = amended
		if cap(cfg.Overrides) == len(cfg.Overrides) {
			t.Fatalf("the shared array has len==cap==%d; without spare capacity this test cannot fail", cap(cfg.Overrides))
		}

		a, b := *cfg, *cfg
		now := base.Add(24 * time.Hour)

		a.AddOverride(Override{Username: "alice", Until: now.Add(time.Hour), Reason: "AMEND-A"})
		b.AddOverride(Override{Username: "alice", Until: now.Add(2 * time.Hour), Reason: "AMEND-B"})

		oa, okA := a.ActiveOverride("alice", now)
		ob, okB := b.ActiveOverride("alice", now)
		if !okA || oa.Reason != "AMEND-A" {
			t.Errorf("a's amended grant reads %q (present %v); b's removal or append reached a's array", oa.Reason, okA)
		}
		if !okB || ob.Reason != "AMEND-B" {
			t.Errorf("b's amended grant reads %q (present %v); a's append reached b's array", ob.Reason, okB)
		}
		// The asymmetry's other half: amending must not duplicate the child, because a
		// second entry for the same username makes which grant wins depend on order.
		for _, tc := range []struct {
			who string
			c   *Config
		}{{"a", &a}, {"b", &b}} {
			var n int
			for _, o := range tc.c.Overrides {
				if o.Username == "alice" {
					n++
				}
			}
			if n != 1 {
				t.Errorf("%s holds %d overrides for alice after one amend; exactly one is expected", tc.who, n)
			}
		}
	})
}

// wideSample is sample() grown enough that every list in the document lands on a
// capacity the doubling allocator rounds up. Two elements decode to cap 2 and would
// prove nothing; three decode to cap 4 and five to cap 8.
func wideSample() *Config {
	c := sample()
	c.Settings.LockBackends = []string{"logind", "kde", "gnome"}
	rules := make([]Rule, 0, 5)
	for i := 0; i < 5; i++ {
		rules = append(rules, Rule{
			ID: fmt.Sprintf("r%d", i), Name: fmt.Sprintf("Rule %d", i),
			Days:    []string{"mon", "tue", "wed", "thu", "fri"},
			Start:   "08:00",
			End:     "13:30",
			Enabled: true,
		})
	}
	c.Profiles = []Profile{
		{ID: "p1", Name: "One", Timezone: "Europe/Berlin", Rules: rules},
		{ID: "p2", Name: "Two", Timezone: "Europe/Berlin", Rules: rules[:2:2]},
		{ID: "p3", Name: "Three", Timezone: "Europe/Berlin", Rules: rules[:3:3]},
	}
	c.Users = []User{
		{Username: "a", ProfileID: "p1", Enabled: true},
		{Username: "b", ProfileID: "p2", Enabled: true},
		{Username: "c", ProfileID: "p3", Enabled: true},
	}
	until := time.Date(2026, 10, 12, 20, 40, 0, 0, time.UTC)
	c.Overrides = []Override{
		{Username: "a", Until: until, Reason: "one"},
		{Username: "b", Until: until, Reason: "two"},
		{Username: "c", Until: until, Reason: "three"},
	}
	return c
}
