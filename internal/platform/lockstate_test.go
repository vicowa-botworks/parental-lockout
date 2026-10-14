package platform

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A backend that cannot see the session must not veto one that can.
//
// lockState used to return the first uncorroborated confirmer's result whatever it was,
// including an error. That error became preErr for the whole Chain.Lock, which discounts
// every later uncorroborated yes — so a read that merely failed made a genuinely locked
// machine report "unconfirmed" forever, and did so with a sentence about a hint that was
// already true rather than the fault that actually happened.
//
// The corroborated half of this function already refuses that shape ("a backend that
// cannot see the session never vetoes one that can see it locked"); this pins that the
// uncorroborated half does too.
func TestAConfirmerThatCannotAnswerDoesNotVetoOneThatCan(t *testing.T) {
	s := desktopSession("3")
	blind := errors.New("no session bus here")

	// first can never answer; second reads the hint and says locked. The hint was already
	// true before the request, so the yes is discounted and the honest answer is
	// "unconfirmed" — but that verdict has to come from the discounting, not from the
	// blind read, and the blind read must still be on the record.
	first := &confirming{Mock: &Mock{BackendName: "first"}, err: blind}
	second := &confirming{Mock: &Mock{BackendName: "second"}, yes: true}
	c := &Chain{backends: []Platform{first, second}}

	err := c.Lock(context.Background(), s)
	if !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("Lock = %v, want ErrUnconfirmed", err)
	}
	if !strings.Contains(err.Error(), "session bus") {
		t.Errorf("the blind backend's fault was swallowed; falling through is not the same as ignoring it. Got: %v", err)
	}
}

// The same property where it changes the answer: the pre-read must not be poisoned by a
// confirmer that errors, because a poisoned pre-read discounts a real confirmation.
//
// The chain is [blind, hint-reader, corroborator]. The corroborator is what actually
// proves the lock, so a correct implementation returns nil; the old one returned
// ErrUnconfirmed with a sentence blaming a hint that was never read.
func TestAPreReadErrorDoesNotDiscountALaterCorroboratedYes(t *testing.T) {
	s := desktopSession("3")
	blind := errors.New("no session bus here")

	first := &confirming{Mock: &Mock{BackendName: "first"}, err: blind}
	hint := &confirming{Mock: &Mock{BackendName: "second"}, yes: false}
	proof := &corroborating{Mock: &Mock{BackendName: "kde"}, yes: true}
	c := &Chain{backends: []Platform{first, hint, proof}}

	if err := c.Lock(context.Background(), s); err != nil {
		t.Fatalf("Lock = %v, want nil: the corroborated backend proved this lock took, and the only thing that failed was a read that could not be made", err)
	}
}
