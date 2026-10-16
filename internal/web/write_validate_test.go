package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

// The write path must refuse to save a document the daemon would refuse to load.
//
// This is not about the override being wrong on its own terms -- the handler already checks
// those. It is about the document as a whole. plock serve exits 1 when Validate fails at
// startup, so a write that leaves the configuration invalid does not merely lose that edit:
// the next restart does not come up and enforcement stops until a human notices. A UI that
// can silently arrange that is worse than one that says no.

// addDanglingUser breaks the document in a way no override handler has an opinion about: an
// account pointing at a profile that does not exist. Validate refuses it; handleOverride
// never looks at it. That gap is the whole point -- the guard cannot be per-handler.
func addDanglingUser(t *testing.T, f *overrideFixture) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.doc.Users = append(f.doc.Users, config.User{
		Username: "ghost", ProfileID: "no-such-profile", Enabled: true,
	})
}

func TestAMutationThatLeavesTheConfigInvalidIsRefusedAndSavesNothing(t *testing.T) {
	f := newOverrideFixture(t)
	addDanglingUser(t, f)

	before := f.savesCount()
	rec := authPost(t, f.s, "/users/casey/override", "minutes=30&reason=homework")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: an invalid document is the request being wrong, "+
			"not the server failing (that would be 503 and would tell the parent to retry): %s",
			rec.Code, rec.Body.String())
	}
	if got := f.savesCount(); got != before {
		t.Errorf("the invalid document was saved anyway: saves went %d -> %d", before, got)
	}
	// The refusal has to carry what Validate objected to, or the parent has a refusal and
	// no way to act on it.
	body := rec.Body.String()
	// Validate names the field path and the offending value, which is what makes the
	// refusal actionable; the username is not in its message and should not be demanded.
	if !strings.Contains(body, "profileID") || !strings.Contains(body, "no-such-profile") {
		t.Errorf("the refusal does not name the offending field; body:\n%s", body)
	}
}

// The same refusal on the clear route, which takes the same path through mutateConfig.
func TestClearingIntoAnInvalidDocumentIsRefusedRatherThanSaved(t *testing.T) {
	f := newOverrideFixture(t)

	authPost(t, f.s, "/users/casey/override", "minutes=30&reason=homework")
	if len(f.overrides()) != 1 {
		t.Fatalf("setup: expected one override, got %d", len(f.overrides()))
	}
	addDanglingUser(t, f)

	before := f.savesCount()
	rec := authPost(t, f.s, "/users/casey/override/clear", "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", rec.Code, rec.Body.String())
	}
	if got := f.savesCount(); got != before {
		t.Errorf("saves went %d -> %d; the clear was written into an invalid document", before, got)
	}
}

// A refusal must not be reported as a successful grant. The audit line and the redirect are
// the grant's evidence, so if either is emitted on a refused write the trail says a child was
// let out when nothing changed.
func TestARefusedWriteWritesNoAuditLine(t *testing.T) {
	f := newOverrideFixture(t)
	addDanglingUser(t, f)

	authPost(t, f.s, "/users/casey/override", "minutes=30&reason=homework")

	if got := strings.Join(f.audited.Lines(), "\n"); strings.Contains(got, "override.granted") {
		t.Errorf("the audit trail records a grant that was refused; trail:\n%s", got)
	}
}
