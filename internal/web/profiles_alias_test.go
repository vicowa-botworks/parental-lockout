package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The rule-add path builds a fresh array before appending, and this pins that it does.
//
// findProfile hands back a copy of the profile, so `p.Rules = ...` is not a write through
// the document's element -- that half is safe by construction. The half that is NOT safe by
// construction is the append itself: a copied slice header still shares the backing array,
// so appending into a profile whose rule slice carries spare capacity writes the handler's
// rule into a slot another reader's snapshot can also reach. internal/config closes this
// with copySlice/appended and pins it for its own helpers (override_alias_test.go); this is
// the same hazard at the one site in this package that appends by hand, and it had no test.
//
// The fixture's rule slice is a composite literal, so it arrives len==cap and the mutation
// would be unobservable; the test therefore gives it room first, the way Load's doubling
// gives the daemon's real slices room, and fails loudly if that room is not there.
func TestAddingARuleDoesNotWriteIntoAnArrayAnotherSnapshotHolds(t *testing.T) {
	f := newOverrideFixture(t)

	f.mu.Lock()
	prof, ok := findProfile(f.doc, "school")
	if !ok {
		f.mu.Unlock()
		t.Fatal("the fixture has no school profile")
	}
	room := make([]config.Rule, len(prof.Rules), len(prof.Rules)+1)
	copy(room, prof.Rules)
	prof.Rules = room
	f.doc.UpsertProfile(prof)

	// A reader that is mid-render: it holds the rule slice as it was before the write.
	held := f.doc.Profiles[0].Rules
	f.mu.Unlock()

	if cap(held) == len(held) {
		t.Fatalf("the shared rule array has len==cap==%d; without spare capacity this test cannot fail", len(held))
	}

	rec := authPost(t, f.s, "/profiles/school/rules",
		"name=Nights&start=20%3A00&end=06%3A00&days=mon")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body:\n%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}

	// The reader finishes by extending its own snapshot. If the handler appended into the
	// shared array, this lands in the same slot the handler just wrote.
	held = append(held, config.Rule{ID: "mine", Name: "Mine",
		Days: []string{"fri"}, Start: "16:00", End: "18:00", Enabled: true})

	f.mu.Lock()
	defer f.mu.Unlock()
	stored, ok := findProfile(f.doc, "school")
	if !ok {
		t.Fatal("the profile is gone")
	}
	if len(stored.Rules) != 2 {
		t.Fatalf("the profile holds %d rules, want 2", len(stored.Rules))
	}
	// The property itself, stated directly: index 1 of the two slices must be two different
	// places. Shared, the reader's append above has already overwritten the stored rule.
	if &held[1] == &stored.Rules[1] {
		t.Errorf("the stored profile and the reader's snapshot share index 1 of one array")
	}
	if stored.Rules[1].Name != "Nights" {
		t.Errorf("the stored rule is %q, not the one the request created: the reader's append wrote into the array the stored profile shares (stored: %s)",
			stored.Rules[1].Name, strings.Join(ruleNames(stored.Rules), ", "))
	}
}

func ruleNames(rules []config.Rule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.Name)
	}
	return out
}
