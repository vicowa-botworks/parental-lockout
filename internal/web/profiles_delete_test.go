package web

import (
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Finding 1: a seam failure must not arrive as "no such profile".
//
// The handler used to end in a catch-all `404 err.Error()`, which caught everything that was
// neither a blocking refusal nor a validation error -- including the errors the seam returns
// when config.Load cannot read the file or Save cannot write it. So a read-only mount or a full
// disk told the operator the profile does not exist: a sentence about the document rather than
// about the failure, and retryable-looking in precisely the wrong direction. A corrupt config
// file was worse, because its parse error was echoed straight into the response body.
//
// The sibling handler on the same PR already got this right, which is what made this one a bug
// rather than a style choice: same seam, same failure, 503.
func TestASeamFailureIsNotReportedAsAMissingProfile(t *testing.T) {
	f := newOverrideFixture(t)
	f.mu.Lock()
	f.failOn = fs.ErrPermission
	f.mu.Unlock()

	before := f.savesCount()
	rec := authPost(t, f.s, "/profiles/school/delete", "")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// The two sentences a disk failure must never be allowed to become.
	if strings.Contains(body, "no profile") || strings.Contains(body, "no such profile") {
		t.Errorf("a seam failure was reported as a missing profile: %q", body)
	}
	// The raw error is logged, not shown: a parse failure names byte offsets and a path.
	if strings.Contains(body, "permission denied") {
		t.Errorf("the raw filesystem error was echoed to the client: %q", body)
	}
	if !strings.Contains(body, "the profile could not be deleted") {
		t.Errorf("the refusal does not say what failed: %q", body)
	}
	if got := f.savesCount(); got != before {
		t.Errorf("saves went %d -> %d on a failed write", before, got)
	}
}

// Finding 2: the profile id comes from the URL, so it must not decide which refusal fires.
//
// The old code recognised the in-use refusal with strings.Contains(err.Error(), "is still
// assigned to"). The comment claimed that avoided matching attacker-shaped input, but the same
// message interpolates the profile id, so the request could supply the matched text itself. An
// id of `x" is still assigned to "y` made a profile that does not exist answer 422 "still
// assigned", with the request's own text reflected back.
//
// Nothing here is exploitable -- pageRefuse goes through http.Error, so the response is
// plain text and no markup can land. The defect is that the server stated a falsehood about
// enforcement ("this profile is in use") because a URL fragment happened to look like one of
// its own sentences.
func TestAHostileProfileIDCannotChooseWhichRefusalFires(t *testing.T) {
	f := newOverrideFixture(t)

	hostile := `x" is still assigned to "y`
	rec := authPost(t, f.s, "/profiles/"+url.PathEscape(hostile)+"/delete", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a profile that does not exist; body %q",
			rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); strings.Contains(body, "still assigned") {
		t.Errorf("a nonexistent profile claimed to be in use, from its own id: %q", body)
	}
	// No reflection of request text at all, not merely no markup.
	if body := rec.Body.String(); strings.Contains(body, hostile) {
		t.Errorf("the requested id was reflected back into the body: %q", body)
	}
}

// The 404 has to stay reachable, or the fix above is just a new name for "everything is a 503".
func TestDeletingAProfileThatDoesNotExistIsNotFound(t *testing.T) {
	f := newOverrideFixture(t)

	before := f.savesCount()
	rec := authPost(t, f.s, "/profiles/never-existed/delete", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %q", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "no profile with that id") {
		t.Errorf("the refusal does not say the profile is absent: %q", body)
	}
	if got := f.savesCount(); got != before {
		t.Errorf("saves went %d -> %d", before, got)
	}
}

// And the genuine refusal still works: in use is 422 and names the accounts, which is the
// answer the operator acts on.
func TestDeletingAProfileInUseIsRefusedAndNamesTheAccount(t *testing.T) {
	f := newOverrideFixture(t)

	before := f.savesCount()
	rec := authPost(t, f.s, "/profiles/school/delete", "")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "casey") {
		t.Errorf("the blocking account was not named: %q", body)
	}
	if got := f.savesCount(); got != before {
		t.Errorf("a refused deletion still saved; saves went %d -> %d", before, got)
	}
}
