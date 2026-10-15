package platform

import (
	"context"
	"testing"
)

// The cache is only a cache if Chain.Lock attaches it.
//
// Every other test in this file calls withLockerCache itself, which proves the mechanism works
// and proves nothing about production: delete the one line in Chain.Lock that attaches it and
// each of those tests still passes, because each hands itself a cache. That is the wiring gap
// this file used to have, and it is the reason mutant K8 (`ctx = withLockerCache(ctx)` replaced
// by `_ = ctx`) survived a green suite.
//
// So this test drives Chain.Lock and asks the backend what it was given. The contract is stated
// at the attach site -- "this is the only place it can be attached" -- and a contract about the
// only place is exactly the kind that needs a test, because nothing else can notice its absence.
func TestChainLockHandsItsBackendsAnOperationScopedCache(t *testing.T) {
	b := &ctxCapturingBackend{Mock: &Mock{BackendName: "ctxprobe"}}

	ch := &Chain{backends: []Platform{b}}
	if err := ch.Lock(context.Background(), desktopSession("2")); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if b.got == nil {
		t.Fatal("the backend was never asked to confirm, so nothing was observed")
	}

	// The context the backend received must carry the operation's cache.
	if c, _ := b.got.Value(cacheKey{}).(*lockerCache); c == nil {
		t.Fatal("Chain.Lock handed its backends a context with no locker cache attached")
	}
}

// The other half of "operation-scoped": two operations must not share one cache. Asserting the
// pointer differs is what makes the test fail if the cache ever moves to package scope, which is
// the shape that would let one operation's YES confirm a later operation's lock.
func TestTwoLockOperationsGetDifferentCaches(t *testing.T) {
	b := &ctxCapturingBackend{Mock: &Mock{BackendName: "ctxprobe"}}
	ch := &Chain{backends: []Platform{b}}

	if err := ch.Lock(context.Background(), desktopSession("2")); err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	first, _ := b.got.Value(cacheKey{}).(*lockerCache)
	if first == nil {
		t.Fatal("first operation had no cache")
	}

	if err := ch.Lock(context.Background(), desktopSession("2")); err != nil {
		t.Fatalf("second Lock: %v", err)
	}
	second, _ := b.got.Value(cacheKey{}).(*lockerCache)
	if second == nil {
		t.Fatal("second operation had no cache")
	}

	if first == second {
		t.Error("two Chain.Lock calls shared one locker cache, so a YES outlives its operation")
	}
}

// ctxCapturingBackend remembers the context it was last asked to confirm with. It corroborates,
// so the chain treats its yes as evidence and stops -- which also means the captured context is
// the one from the operation that succeeded, not a later retry.
type ctxCapturingBackend struct {
	*Mock
	got context.Context
}

func (b *ctxCapturingBackend) LockConfirmed(ctx context.Context, s Session) (bool, error) {
	b.got = ctx
	return true, nil
}

func (b *ctxCapturingBackend) Corroborated() bool { return true }
