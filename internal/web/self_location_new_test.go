package web

// The constructor is the documented way to build a Server, so a dependency that only
// arrives by assigning the field afterwards is a dependency the shipped daemon never
// gets. This is the same trap the asset map was caught in (W44): a fixture that assigns
// the field keeps the suite green while New silently drops it.

import (
	"testing"

	"github.com/vicowa-botworks/parental-lockout/internal/platform"
)

func TestNewHandsTheSelfLocationToTheServer(t *testing.T) {
	// Distinctive on both halves. A zero value in and a zero value out would pass when
	// the constructor drops the field, because dropping it and forwarding it are
	// indistinguishable at zero -- and zero is what a systemd unit gets anyway, so the
	// drop would stay invisible in the deployment where the rule never fires and fatal
	// in the one where it does.
	want := platform.SelfLocation{UID: 4123, InUserSlice: true}

	s, err := New(Options{
		StoreDir: t.TempDir(),
		TLSMode:  TLSOff,
		Listen:   "127.0.0.1:0",
		Self:     want,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.Self != want {
		t.Errorf("New built a server with Self = %+v, want %+v; the enforcement loop knows it "+
			"is inside uid 4123's login session and the dashboard does not, so the page reports "+
			"that account as locked out while the loop declines to lock it", s.Self, want)
	}
}

// The default, checked rather than assumed: a caller that says nothing gets the answer
// that keeps enforcement on. If this ever came back "inside uid 0's slice", every daemon
// built without the field would stop enforcing root's sessions and nothing would say so.
func TestAServerThatWasToldNothingDeclinesNobody(t *testing.T) {
	s, err := New(Options{
		StoreDir: t.TempDir(),
		TLSMode:  TLSOff,
		Listen:   "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.Self.Declines(0) || s.Self.InUserSlice {
		t.Errorf("a server assembled without Self reports %+v; the zero value must decline "+
			"nobody, because that is what a systemd system unit is", s.Self)
	}
}
