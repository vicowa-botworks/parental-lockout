package dashboard

import (
	"github.com/vicowa-botworks/parental-lockout/internal/platform"
	"testing"
)

// The other half of TestAnExpiredOverrideIsShownAsExpired.
//
// Saying "your unlock ran out" to a parent who never unlocked anyone is the same lie
// told from the other side: it sends them looking for an override that does not exist
// instead of looking at the schedule that is actually holding the account. The expired
// flag has to be false for the overwhelming majority of rows, which are accounts that
// simply have no override.
func TestARowWithNoOverrideClaimsNone(t *testing.T) {
	v := Build(cfg(casey(true)), nil, platform.SelfLocation{}, insideWindow)
	if len(v.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(v.Rows))
	}
	r := v.Rows[0]
	if r.OverrideExpired {
		t.Error("an account with no override at all is reported as having had one that expired")
	}
	if r.OverrideActive {
		t.Error("an account with no override is reported as released by one")
	}
	if !r.OverrideUntil.IsZero() {
		t.Errorf("an account with no override carries an until time: %v", r.OverrideUntil)
	}
}
