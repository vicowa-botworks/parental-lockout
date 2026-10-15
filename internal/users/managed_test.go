package users

import "testing"

func acct(name string) Account {
	return Account{Username: name, UID: 1000, Home: "/home/" + name, Shell: "/bin/bash"}
}
func conf(name string, enabled bool) ConfiguredUser {
	return ConfiguredUser{Username: name, ProfileID: "school", Enabled: enabled}
}

func row(t *testing.T, rows []Managed, name string) Managed {
	t.Helper()
	for _, r := range rows {
		if r.Username == name {
			return r
		}
	}
	t.Fatalf("no row for %q in %v", name, order(rows))
	return Managed{}
}

func order(rows []Managed) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Username)
	}
	return out
}

// The whole point of the join: one row per account, carrying both answers.
func TestUnionPutsBothSourcesOnTheSameRow(t *testing.T) {
	rows := Union([]Account{acct("casey")}, []ConfiguredUser{conf("casey", true)})
	if len(rows) != 1 {
		t.Fatalf("one account in both sources produced %d rows", len(rows))
	}
	r := rows[0]
	if r.OnMachine == nil || r.Configured == nil {
		t.Fatalf("a row lost one of its sources: %+v", r)
	}
	if r.Stale {
		t.Error("an account that is on the machine is not stale")
	}
	if r.Configured.ProfileID != "school" || !r.Configured.Enabled {
		t.Errorf("the config half was not carried through: %+v", r.Configured)
	}
}

// A child who has never been added must appear, or the screen cannot offer the one
// action that matters.
func TestUnionShowsAnUnmanagedAccount(t *testing.T) {
	rows := Union([]Account{acct("casey")}, nil)
	r := row(t, rows, "casey")
	if r.OnMachine == nil {
		t.Fatal("the discovered half is missing")
	}
	if r.Configured != nil {
		t.Error("an account nobody has configured cannot have a config half")
	}
	if r.Stale {
		t.Error("it exists on the machine; calling it stale sends the parent to the wrong problem")
	}
}

// The other half, and the one a password-file-only screen cannot show: configured,
// gone from the machine. It has to stay visible, because a row that vanishes cannot
// be un-assigned from.
func TestUnionKeepsAConfiguredAccountThatIsGoneFromTheMachine(t *testing.T) {
	rows := Union(nil, []ConfiguredUser{conf("quinn", true)})
	r := row(t, rows, "quinn")
	if r.OnMachine != nil {
		t.Error("the machine does not have this account")
	}
	if r.Configured == nil {
		t.Fatal("dropping the config half would make the row unreachable")
	}
	if !r.Stale {
		t.Error("the row must say it cannot be locked out any more")
	}
}

// Ordering is by name and covers every row from both sides. The screen paginates, so
// a row that moves between loads is worse than a slow load.
func TestUnionOrdersByUsernameAcrossBothSources(t *testing.T) {
	rows := Union(
		[]Account{acct("zed"), acct("amy"), acct("mo")},
		[]ConfiguredUser{conf("yuki", true), conf("amy", false)},
	)
	got := order(rows)
	want := []string{"amy", "mo", "yuki", "zed"}
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// A duplicate in the config is a broken document, but the screen still has to pick
// one deterministically rather than show whichever the map happened to visit.
func TestUnionResolvesADuplicateConfigEntryDeterministically(t *testing.T) {
	rows := Union(nil, []ConfiguredUser{
		conf("casey", false),
		{Username: "casey", ProfileID: "weekend", Enabled: true},
	})
	if len(rows) != 1 {
		t.Fatalf("a duplicated name produced %d rows, want 1", len(rows))
	}
	// Last write wins, which is what a caller reading a document top to bottom
	// expects. What matters is that it is not arbitrary.
	if rows[0].Configured.ProfileID != "weekend" || !rows[0].Configured.Enabled {
		t.Errorf("the duplicate resolved to %+v", rows[0].Configured)
	}
}

// The rows must be snapshots. The config layer is copy-on-write, so a pointer into
// the slice passed here would keep a document alive that nothing still calls
// current, and a page rendered from it would disagree with the file it claims to
// show.
func TestUnionRowsDoNotAliasTheCallersSlices(t *testing.T) {
	discovered := []Account{acct("casey")}
	configured := []ConfiguredUser{conf("casey", true)}
	rows := Union(discovered, configured)

	discovered[0].Home = "/elsewhere"
	configured[0].Enabled = false
	configured[0].ProfileID = "changed"

	r := rows[0]
	if r.OnMachine.Home != "/home/casey" {
		t.Errorf("the machine half aliases the caller's slice: %q", r.OnMachine.Home)
	}
	if !r.Configured.Enabled || r.Configured.ProfileID != "school" {
		t.Errorf("the config half aliases the caller's slice: %+v", r.Configured)
	}
}

// An account that exists and is not enabled is still work to do. Counting "has a
// config row" as managed under-reports on exactly the machine where nothing has been
// done yet.
func TestUnmanagedCountsDisabledEntriesAsStillToDo(t *testing.T) {
	rows := Union(
		[]Account{acct("never"), acct("disabled"), acct("active")},
		[]ConfiguredUser{conf("disabled", false), conf("active", true)},
	)
	if got := Unmanaged(rows); got != 2 {
		t.Errorf("Unmanaged = %d, want 2 (never configured + configured but off); rows %v", got, order(rows))
	}
}

// A stale row that is still enabled is not "work to do" in the same sense, but it is
// not managed either: it cannot be enforced. It counts, so the number does not claim
// the machine is covered.
func TestUnmanagedCountsAStaleEnabledRow(t *testing.T) {
	rows := Union(nil, []ConfiguredUser{conf("ghost", true)})
	if got := Unmanaged(rows); got != 1 {
		t.Errorf("Unmanaged = %d, want 1: a row that cannot be enforced is not coverage", got)
	}
}

func TestUnionOfTwoEmptiesIsEmpty(t *testing.T) {
	rows := Union(nil, nil)
	if len(rows) != 0 {
		t.Errorf("got %v", order(rows))
	}
	if Unmanaged(rows) != 0 {
		t.Error("nothing to manage is not one thing to manage")
	}
}
