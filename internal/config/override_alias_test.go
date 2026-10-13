package config

import (
	"path/filepath"
	"testing"
	"time"
)

// Two holders of one array both grant an unlock.
//
// This is the shape the rest of this file's aliasing tests miss, and the reason is
// structural rather than careless: one snapshot plus one append is harmless, because
// the write lands past the snapshot's length and nothing that reads through len can
// see it. It takes two configs sharing the same array each appending, so the second
// write lands in the spare capacity the first one was about to use.
//
// The spare capacity is not hypothetical. encoding/json grows slices by doubling, so
// a document with three overrides decodes to len 3, cap 4 — and Load is the only way
// the daemon obtains a Config, so every poll starts with exactly this array.
//
// The direction is what makes it worth a test of its own: the grant that vanishes
// fails closed and reads as a bug someone reports, while the grant that appears from
// nowhere unlocks a child nobody unlocked. That is the one outcome this package's
// fail-safe rule exists to prevent.
func TestAddOverrideDoesNotWriteIntoAnArrayAnotherConfigHolds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	src := sample()
	base := time.Date(2026, 10, 12, 20, 40, 0, 0, time.UTC)
	src.Overrides = []Override{
		{Username: "one", Until: base, Reason: "existing"},
		{Username: "two", Until: base, Reason: "existing"},
		{Username: "three", Until: base, Reason: "existing"},
	}
	if err := src.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// The precondition, asserted rather than assumed: without spare capacity the
	// append allocates on its own and the test proves nothing. Checked on the slice
	// Load returned, not through any helper that could hide the sharing.
	if got := cap(cfg.Overrides); got == len(cfg.Overrides) {
		t.Fatalf("Load returned len==cap==%d; encoding/json was expected to leave spare capacity, and without it this test cannot fail", got)
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
}
