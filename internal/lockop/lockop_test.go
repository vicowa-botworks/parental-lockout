package lockop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

// The three ways a lock can end, and the accounting that keeps them apart.
//
// This package exists because the difference between "the screen is locked", "we asked and
// nothing can prove it" and "nothing reached the desktop" is the product: two callers, a
// terminal and a web page, render the same event, and they are only allowed to say the same
// sentence if one place decided which of the three happened. So every test here asserts on
// which field is set and which counter moved, not on a summary word -- a caller that reads
// Locked would otherwise never notice that Unconfirmed was the truth.
//
// The refusals are tested against the fake as well as the Result: a session that must not be
// locked has to be counted AND untouched, and "counted but still locked" is exactly as wrong
// as "locked and reported as refused".

// lockable is a session that passes IsLockable. Every test that is about the lock rather
// than about the refusal has to start from one, or it silently asserts on the refusal.
func lockable(id string, user string) platform.Session {
	return platform.Session{ID: id, Username: user, UID: 1000,
		Class: "user", Seat: "seat0", Type: "x11", State: "active"}
}

// fakeChain states which of the three answers the machine gave, per session.
type fakeChain struct {
	mu       sync.Mutex
	asked    []string
	answers  map[string]error
	listErr  error
	sessions []platform.Session
}

func (f *fakeChain) Sessions(context.Context) ([]platform.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.sessions, nil
}

func (f *fakeChain) Lock(_ context.Context, s platform.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, s.ID)
	// An absent answer means nil on purpose. The tests that care about a session that did
	// NOT lock name it; a default of "failed" would let a test that forgot to wire an
	// answer pass by accident.
	return f.answers[s.ID]
}

func (f *fakeChain) askedFor() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.asked...)
}

func newChain(sessions []platform.Session, answers map[string]error) *fakeChain {
	return &fakeChain{sessions: sessions, answers: answers}
}

// TestAConfirmedLockIsTheOnlyOutcomeThatSaysLocked. A nil from the chain means a backend
// proved the screen went down (#965: the chain only stops at a backend that can corroborate),
// and this is the one case that may set Locked.
func TestAConfirmedLockIsTheOnlyOutcomeThatSaysLocked(t *testing.T) {
	sess := lockable("2", "casey")
	res := Now(context.Background(), newChain([]platform.Session{sess}, nil), []platform.Session{sess})

	if len(res.Outcomes) != 1 {
		t.Fatalf("one session produced %d outcomes", len(res.Outcomes))
	}
	o := res.Outcomes[0]
	if !o.Locked {
		t.Error("a confirmed lock did not set Locked")
	}
	if o.Unconfirmed {
		t.Error("a confirmed lock also set Unconfirmed; the two are different sentences")
	}
	if o.Detail != "" {
		t.Errorf("a lock that worked has nothing to explain, got %q", o.Detail)
	}
	if res.Locked != 1 || res.Unconfirmed != 0 || res.Refused != 0 {
		t.Errorf("counts are locked %d, unconfirmed %d, refused %d; want 1/0/0",
			res.Locked, res.Unconfirmed, res.Refused)
	}
	if !res.Asked() {
		t.Error("a request that locked a session reports that nothing was asked")
	}
}

// TestAnUnconfirmedAskIsNeitherLockedNorFailed is the raw-virtual-terminal case: the ask went
// through, the exit status was zero, and nothing on the machine can attest that the screen is
// dark. Collapsing it into Locked is the lie; collapsing it into a failure is the other one,
// because it would tell the operator to go fix a backend that did its job.
func TestAnUnconfirmedAskIsNeitherLockedNorFailed(t *testing.T) {
	sess := lockable("2", "casey")
	res := Now(context.Background(),
		newChain([]platform.Session{sess}, map[string]error{"2": platform.ErrUnconfirmed}),
		[]platform.Session{sess})

	o := res.Outcomes[0]
	if o.Locked {
		t.Error("an unconfirmed ask set Locked; the page would say the screen is down")
	}
	if !o.Unconfirmed {
		t.Error("an unconfirmed ask did not set Unconfirmed")
	}
	if o.Detail == "" {
		t.Error("the outcome carries no explanation of what could not be proved")
	}
	if res.Locked != 0 || res.Unconfirmed != 1 {
		t.Errorf("counts are locked %d, unconfirmed %d; want 0/1", res.Locked, res.Unconfirmed)
	}
}

// TestAnUnconfirmedAskWrappedByTheChainStillReadsAsUnconfirmed. platform.Chain.Lock joins the
// errors of the backends that could not corroborate, so the value that reaches here is often
// a wrapper rather than the sentinel. errors.Is is what makes that the same answer, and a
// == comparison would silently reclassify every real unconfirmed lock as a hard failure.
func TestAnUnconfirmedAskWrappedByTheChainStillReadsAsUnconfirmed(t *testing.T) {
	sess := lockable("2", "casey")
	wrapped := fmt.Errorf("gdbus: %w", platform.ErrUnconfirmed)
	res := Now(context.Background(),
		newChain([]platform.Session{sess}, map[string]error{"2": wrapped}),
		[]platform.Session{sess})

	if !res.Outcomes[0].Unconfirmed {
		t.Errorf("a wrapped ErrUnconfirmed was classified as something else: %+v", res.Outcomes[0])
	}
	if res.Locked != 0 {
		t.Error("a wrapped ErrUnconfirmed counted as locked")
	}
}

// TestAHardErrorIsNeitherLockedNorUnconfirmed. Nothing reached the desktop -- no session
// manager, no bus, a binary that could not be found. That is a different sentence from "asked
// and cannot prove", and it must not borrow the unconfirmed counter, because the operator's
// next step is different in each case.
func TestAHardErrorIsNeitherLockedNorUnconfirmed(t *testing.T) {
	sess := lockable("2", "casey")
	boom := errors.New("gdbus: cannot connect to bus")
	res := Now(context.Background(),
		newChain([]platform.Session{sess}, map[string]error{"2": boom}),
		[]platform.Session{sess})

	o := res.Outcomes[0]
	if o.Locked || o.Unconfirmed {
		t.Errorf("a hard error set a positive field: %+v", o)
	}
	if o.Detail != boom.Error() {
		t.Errorf("the outcome does not carry the reason: %q", o.Detail)
	}
	if res.Locked != 0 || res.Unconfirmed != 0 || res.Refused != 0 {
		t.Errorf("a hard error moved a counter: %d/%d/%d, want 0/0/0",
			res.Locked, res.Unconfirmed, res.Refused)
	}
	if !res.Asked() {
		t.Error("a session that was asked and failed reports that nothing was asked")
	}
}

// TestASessionThatCannotBeLockedIsCountedAndNeverTouched. A greeter or a manager session shares
// the uid with the real one, so the loop has to refuse it by name and say why. Silently
// skipping it would produce "locked 0 of 1" with no reason, which reads as a broken backend.
func TestASessionThatCannotBeLockedIsCountedAndNeverTouched(t *testing.T) {
	greeter := platform.Session{ID: "1", Username: "casey", UID: 1000,
		Class: "greeter", Seat: "seat0", Type: "x11"}
	c := newChain([]platform.Session{greeter}, nil)
	res := Now(context.Background(), c, []platform.Session{greeter})

	if got := c.askedFor(); len(got) != 0 {
		t.Errorf("a greeter session was handed to the backend: %v", got)
	}
	if res.Refused != 1 {
		t.Errorf("an unlockable session is not in the refused count: %+v", res)
	}
	if res.Locked != 0 || res.Unconfirmed != 0 {
		t.Error("an unlockable session counted as a lock of some kind")
	}
	if len(res.Outcomes) != 1 {
		t.Fatalf("the refusal produced %d outcomes; it has to be on the list to be said",
			len(res.Outcomes))
	}
	if !strings.Contains(res.Outcomes[0].Detail, "greeter") {
		t.Errorf("the refusal does not name what kind of session it was: %q",
			res.Outcomes[0].Detail)
	}
	if strings.Contains(res.Outcomes[0].Detail, "x11") {
		t.Errorf("the refusal blames the session type, which the child's real desktop shares: %q",
			res.Outcomes[0].Detail)
	}
}

// TestARemoteRefusalSaysRemote. The remote case is the loud one in platform's rule -- locking
// it shuts the admin out of the machine -- and it is the one where "class user" would be
// actively unhelpful, because the class is exactly what a remote admin session has.
func TestARemoteRefusalSaysRemote(t *testing.T) {
	sess := lockable("7", "admin")
	sess.Remote = true
	c := newChain([]platform.Session{sess}, nil)
	res := Now(context.Background(), c, []platform.Session{sess})

	if res.Refused != 1 {
		t.Fatalf("a remote session was not refused: %+v", res)
	}
	if got := c.askedFor(); len(got) != 0 {
		t.Errorf("a remote session was handed to the backend: %v", got)
	}
	if !strings.Contains(res.Outcomes[0].Detail, "remote") {
		t.Errorf("the refusal does not say the session is remote: %q", res.Outcomes[0].Detail)
	}
}

// TestARefusalWithNoReadableCauseInventsNothing. A user-class session with no seat is refused by
// the rule, but nothing on the record says "no seat" is why -- so the sentence stays the plain
// house one. A refusal that named a cause it had not read is the same failure as a lock result
// that named a verdict the machine could not prove.
func TestARefusalWithNoReadableCauseInventsNothing(t *testing.T) {
	sess := lockable("9", "casey")
	sess.Seat = ""
	res := Now(context.Background(), newChain([]platform.Session{sess}, nil), []platform.Session{sess})

	if res.Refused != 1 {
		t.Fatalf("a seatless session was not refused: %+v", res)
	}
	if got := res.Outcomes[0].Detail; got != "not a lockable desktop session" {
		t.Errorf("the refusal added a cause nobody read: %q", got)
	}
}

// TestOneDeadBackendDoesNotSwallowTheSessionsAfterIt. Now returns no error for a failed lock on
// purpose: a caller that got one would report nothing, and the operator would lose the
// outcomes of the sessions that did lock.
func TestOneDeadBackendDoesNotSwallowTheSessionsAfterIt(t *testing.T) {
	bad := lockable("2", "casey")
	good := lockable("3", "casey")
	c := newChain([]platform.Session{bad, good},
		map[string]error{"2": errors.New("no session bus")})
	res := Now(context.Background(), c, []platform.Session{bad, good})

	if got := c.askedFor(); len(got) != 2 {
		t.Errorf("the loop stopped early; the backend saw %v", got)
	}
	if len(res.Outcomes) != 2 {
		t.Fatalf("two sessions produced %d outcomes", len(res.Outcomes))
	}
	if res.Locked != 1 {
		t.Errorf("the session after the failure was not counted as locked: %+v", res)
	}
	if res.Outcomes[0].Locked {
		t.Error("the failed session claims to be locked")
	}
}

// TestAnAccountWithNoSessionIsNotAnError. "Logged out" is a state the caller has to be able to
// say, and it is a different sentence from "no such account" and from "the listing failed". An
// error here would push the caller to render the one failure sentence for all three.
func TestAnAccountWithNoSessionIsNotAnError(t *testing.T) {
	sess := lockable("2", "casey")
	res, err := SessionsFor(context.Background(), newChain([]platform.Session{sess}, nil), "mallory")
	if err != nil {
		t.Fatalf("an account with no session returned an error: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("another account's session was handed back: %+v", res)
	}
	// And the Result of locking nothing says so without claiming anything.
	if (Result{}).Asked() {
		t.Error("the zero Result reports that a session was touched")
	}
}

// TestAFailedListingIsTheOneError. "I could not look" must not arrive as an empty list, which
// is the same fail-open shape the audit page refuses: an empty listing is a reassurance, an
// error is a fact.
func TestAFailedListingIsTheOneError(t *testing.T) {
	boom := errors.New("loginctl: signal: killed")
	c := newChain(nil, nil)
	c.listErr = boom

	got, err := SessionsFor(context.Background(), c, "casey")
	if !errors.Is(err, boom) {
		t.Errorf("the listing failure did not reach the caller: %v", err)
	}
	if got != nil {
		t.Errorf("a failed listing returned sessions anyway: %+v", got)
	}
}

// TestOwnedByTakesEverySessionOfThatAccountExactly. An account can hold a greeter row and a
// desktop row at once, so this is a filter that keeps both and drops everything else -- and the
// match is exact, because two accounts whose names differ only by case are two accounts.
func TestOwnedByTakesEverySessionOfThatAccountExactly(t *testing.T) {
	all := []platform.Session{
		{ID: "1", Username: "Casey"},
		{ID: "2", Username: "casey"},
		{ID: "3", Username: "casey"},
		{ID: "4", Username: "casey2"},
	}
	got := OwnedBy(all, "casey")
	// The ids, not just the count: an inverted filter returns two rows as well, and a test
	// that only counted them would call it correct.
	var ids []string
	for _, s := range got {
		ids = append(ids, s.ID)
		if s.Username != "casey" {
			t.Errorf("OwnedBy returned %q for %q", s.Username, "casey")
		}
	}
	if strings.Join(ids, ",") != "2,3" {
		t.Fatalf("OwnedBy returned sessions %v, want exactly 2 and 3: %+v", ids, got)
	}
	if OwnedBy(nil, "casey") != nil {
		t.Error("an empty listing returned sessions")
	}
}
