package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestUpsertProfileAddsThenReplaces(t *testing.T) {
	c := Default()

	c.UpsertProfile(Profile{ID: "school", Name: "School", Timezone: "UTC"})
	if len(c.Profiles) != 1 {
		t.Fatalf("profiles = %d, want 1", len(c.Profiles))
	}

	c.UpsertProfile(Profile{ID: "school", Name: "School hours", Timezone: "Europe/Berlin"})
	if len(c.Profiles) != 1 {
		t.Fatalf("upserting the same id added a profile: %d", len(c.Profiles))
	}
	if c.Profiles[0].Name != "School hours" || c.Profiles[0].Timezone != "Europe/Berlin" {
		t.Errorf("profile not replaced: %+v", c.Profiles[0])
	}

	c.UpsertProfile(Profile{ID: "weekend", Name: "Weekend", Timezone: "UTC"})
	if len(c.Profiles) != 2 {
		t.Errorf("profiles = %d, want 2", len(c.Profiles))
	}
}

func TestDeleteProfileRefusesWhileAssigned(t *testing.T) {
	c := valid()

	err := c.DeleteProfile("schoolnight")
	if err == nil {
		t.Fatal("DeleteProfile removed a profile still assigned to an account")
	}
	if !strings.Contains(err.Error(), "kiddo") {
		t.Errorf("error = %v, want it to name the account holding the profile", err)
	}
	if len(c.Profiles) != 1 {
		t.Errorf("the refused delete still changed the list: %d profiles", len(c.Profiles))
	}

	c.RemoveUser("kiddo")
	if err := c.DeleteProfile("schoolnight"); err != nil {
		t.Fatalf("DeleteProfile after reassigning: %v", err)
	}
	if len(c.Profiles) != 0 {
		t.Errorf("profiles = %v, want none", c.Profiles)
	}

	if err := c.DeleteProfile("schoolnight"); err == nil {
		t.Error("DeleteProfile of a nonexistent profile succeeded")
	}
}

func TestUpsertUserAssignsOneProfile(t *testing.T) {
	c := valid()

	c.UpsertUser(User{Username: "kiddo", ProfileID: "schoolnight", Enabled: false})
	if len(c.Users) != 1 {
		t.Fatalf("users = %d, want 1", len(c.Users))
	}
	if c.Users[0].Enabled {
		t.Error("upsert did not replace the existing assignment")
	}

	c.UpsertUser(User{Username: "second", ProfileID: "schoolnight", Enabled: true})
	if len(c.Users) != 2 {
		t.Errorf("users = %d, want 2", len(c.Users))
	}
}

func TestRemoveUserDropsItsOverride(t *testing.T) {
	c := valid()
	c.AddOverride(Override{Username: "kiddo", Until: time.Now().Add(time.Hour), Reason: "homework", By: "parent"})
	c.AddOverride(Override{Username: "other", Until: time.Now().Add(time.Hour)})

	if err := c.RemoveUser("kiddo"); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
	if _, ok := c.ActiveOverride("kiddo", time.Now()); ok {
		t.Error("the removed account kept its override")
	}
	if _, ok := c.ActiveOverride("other", time.Now()); !ok {
		t.Error("removing one account dropped another account's override")
	}
	if err := c.RemoveUser("kiddo"); err == nil {
		t.Error("removing an account twice succeeded the second time")
	}
}

func TestAddOverrideNormalizesTheDeadline(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("no timezone database: %v", err)
	}

	c := valid()
	c.AddOverride(Override{Username: "kiddo", Until: time.Date(2026, 10, 12, 20, 40, 0, 0, berlin)})

	got, ok := c.ActiveOverride("kiddo", time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("the override is not active")
	}
	if got.Until.Location() != time.UTC {
		t.Errorf("deadline kept its %v zone; the caller's value would not survive a round trip", got.Until.Location())
	}
	if want := "2026-10-12T18:40:00Z"; got.Until.Format(time.RFC3339) != want {
		t.Errorf("deadline = %s, want %s", got.Until.Format(time.RFC3339), want)
	}
}

func TestUserForAndProfileFor(t *testing.T) {
	c := valid()

	if u, ok := c.UserFor("kiddo"); !ok || u.ProfileID != "schoolnight" {
		t.Errorf("UserFor(kiddo) = %+v, %v", u, ok)
	}
	if _, ok := c.UserFor("ghost"); ok {
		t.Error("UserFor found an account that does not exist")
	}

	p, ok := c.ProfileFor("kiddo")
	if !ok || p.ID != "schoolnight" {
		t.Fatalf("ProfileFor(kiddo) = %+v, %v", p, ok)
	}

	c.Users[0].Enabled = false
	if _, ok := c.ProfileFor("kiddo"); ok {
		t.Error("ProfileFor returned a profile for an account enforcement is off for")
	}
	c.Users[0].Enabled = true

	c.Users[0].ProfileID = "deleted-long-ago"
	if _, ok := c.ProfileFor("kiddo"); ok {
		t.Error("ProfileFor returned a profile for a dangling profileID")
	}

	if _, ok := c.ProfileFor("ghost"); ok {
		t.Error("ProfileFor returned a profile for an unknown account")
	}
}

func TestOverridesAreOnePerAccount(t *testing.T) {
	c := valid()
	now := time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)

	c.AddOverride(Override{Username: "kiddo", Until: now.Add(30 * time.Minute), Reason: "first", By: "parent"})
	c.AddOverride(Override{Username: "kiddo", Until: now.Add(time.Hour), Reason: "second", By: "parent"})

	if len(c.Overrides) != 1 {
		t.Fatalf("overrides = %d, want 1 — a second grant must replace the first", len(c.Overrides))
	}

	got, ok := c.ActiveOverride("kiddo", now)
	if !ok || got.Reason != "second" {
		t.Errorf("ActiveOverride = %+v, %v, want the second grant", got, ok)
	}

	// The grant has to stop mattering the instant it expires, not on the next
	// save or reload.
	if _, ok := c.ActiveOverride("kiddo", now.Add(time.Hour)); ok {
		t.Error("an override whose deadline has passed is still active")
	}
	if _, ok := c.ActiveOverride("nobody", now); ok {
		t.Error("ActiveOverride matched a different account")
	}

	if !c.RemoveOverride("kiddo") {
		t.Error("RemoveOverride reported nothing to remove")
	}
	if c.RemoveOverride("kiddo") {
		t.Error("RemoveOverride reported a second removal")
	}
}

// Save normalizes before encoding, and normalizing must not reach back into the
// caller's value: the web UI keeps the submitted config around to show the user
// what they typed, and Save mutates the copy it works on.
func TestNormalizedDoesNotAliasTheReceiver(t *testing.T) {
	in := &Config{
		Admin:     Admin{Username: "parent"},
		Users:     []User{{Username: "kiddo", ProfileID: "p"}},
		Overrides: []Override{{Username: "kiddo", Reason: "first"}},
		Profiles: []Profile{{
			ID:       "p",
			Name:     "School",
			Timezone: "UTC",
			Rules:    []Rule{{ID: "r", Days: []string{"mon", "tue"}}},
		}},
		Settings: Settings{LockBackends: []string{"logind"}},
	}

	got := in.normalized()

	got.Settings.LockBackends[0] = "changed"
	got.Profiles[0].Name = "REPLACED"
	got.Profiles[0].Rules[0].ID = "changed"
	got.Profiles[0].Rules[0].Days[0] = "changed"
	got.Users[0].Username = "changed"
	got.Overrides[0].Reason = "changed"
	// Deleting through the copy must not shift the caller's backing array.
	got.Profiles = got.Profiles[:0]
	got.Users = got.Users[:0]

	if in.Settings.LockBackends[0] != "logind" {
		t.Errorf("LockBackends aliased: %v", in.Settings.LockBackends)
	}
	if in.Profiles[0].Name != "School" {
		t.Errorf("Profiles aliased: %+v", in.Profiles[0])
	}
	if in.Profiles[0].Rules[0].ID != "r" {
		t.Errorf("Rules aliased: %+v", in.Profiles[0].Rules[0])
	}
	if in.Profiles[0].Rules[0].Days[0] != "mon" {
		t.Errorf("Rule.Days aliased: %v", in.Profiles[0].Rules[0].Days)
	}
	if in.Users[0].Username != "kiddo" {
		t.Errorf("Users aliased: %v", in.Users)
	}
	if in.Overrides[0].Reason != "first" {
		t.Errorf("Overrides aliased: %v", in.Overrides)
	}
	if len(in.Profiles) != 1 || len(in.Users) != 1 {
		t.Errorf("truncating the copy truncated the receiver: %d profiles, %d users", len(in.Profiles), len(in.Users))
	}
}

// Normalizing must not fill in the caller's blanks either — that is the
// difference between "here is what will be written" and "your value changed".
func TestNormalizedDoesNotMutateTheReceiver(t *testing.T) {
	c := &Config{Admin: Admin{Username: "parent"}}

	got := c.normalized()
	if c.Settings.PollSeconds != 0 {
		t.Errorf("PollSeconds on the receiver = %d, want it left at 0", c.Settings.PollSeconds)
	}
	if c.Settings.LockBackends != nil {
		t.Errorf("LockBackends on the receiver = %v, want it left nil", c.Settings.LockBackends)
	}
	if got.Settings.PollSeconds != DefaultPollSeconds {
		t.Errorf("normalized PollSeconds = %d, want the default", got.Settings.PollSeconds)
	}
}

// nil and empty are different documents — one writes `null`, the other `[]` —
// and the copy must not collapse them, or the round trip would not be an
// identity for a config that has an empty-but-present list.
func TestNormalizedPreservesNilVersusEmpty(t *testing.T) {
	empty := &Config{Users: []User{}, Profiles: []Profile{}}
	if got := empty.normalized(); got.Users == nil || got.Profiles == nil {
		t.Errorf("an empty list became nil: %+v", got)
	}

	nilLists := &Config{}
	if got := nilLists.normalized(); got.Users != nil || got.Profiles != nil {
		t.Errorf("a nil list became empty: %+v", got)
	}
}

func TestDefaultIsValidApartFromTheAdmin(t *testing.T) {
	c := Default()
	if c.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", c.Version, CurrentVersion)
	}
	if len(c.Profiles) != 0 || len(c.Users) != 0 || len(c.Overrides) != 0 {
		t.Errorf("Default has content: %+v", c)
	}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "admin.username") {
		t.Errorf("Validate on Default = %v, want only the missing administrator", err)
	}
	if strings.Count(err.Error(), "; ") != 0 {
		t.Errorf("Validate on Default reported more than the missing administrator: %v", err)
	}
}

func TestApplyDefaultsIsIdempotent(t *testing.T) {
	once := Default()
	twice := once.normalized()
	thrice := twice.normalized()

	if !reflect.DeepEqual(once, twice) || !reflect.DeepEqual(twice, thrice) {
		t.Errorf("normalizing twice changed the document:\n%+v\n%+v\n%+v", once, twice, thrice)
	}
}
