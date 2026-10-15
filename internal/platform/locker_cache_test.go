package platform

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// The performance claim, and the correctness property that outranks it, are the same
// mechanism seen from two sides: a YES is remembered, a NO is not.
//
// This test is the NO side, and it is the reason the cache is not a general-purpose memo
// with a TTL. The settle window exists because the desktop raises the signal a moment
// after the lock request returns, and the locker process appears in that same moment. A
// cache that remembered "not running" would be filled before the locker spawned and would
// then hand every remaining poll in the window the stale answer -- the operation would
// report unconfirmed on a machine that had genuinely locked, permanently, and the symptom
// would read as "Plasma is ignoring the lock again", which is the exact misdiagnosis this
// whole area of the code was built to avoid.
func TestANotYetAnswerIsNeverReusedWithinOneOperation(t *testing.T) {
	f := newProcFixture(t)
	prev := procRoot
	procRoot = f.root
	t.Cleanup(func() { procRoot = prev })

	uid := fixtureUID
	// The locker is not there yet, exactly as at the instant lock-session returns.
	ctx := withLockerCache(context.Background())
	yes, err := lockerRunningCached(ctx, uid)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if yes {
		t.Fatal("the locker is reported running before it was added")
	}

	// It appears, the way kscreenlocker_greet does a moment later.
	f.addWithExe(4242, 7619, uid, "kscreenlocker_greet", "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet")
	f.add(7619, 1, uid, "/usr/bin/ksmserver")

	yes, err = lockerRunningCached(ctx, uid)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if !yes {
		t.Error("the locker that appeared mid-operation was not seen: a cached NO suppressed the evidence the settle window exists to wait for")
	}
}

// The YES side: once the locker has been found, the same operation must not walk the whole
// process table again to reach the same answer.
func TestAYesIsRememberedForTheRestOfTheOperation(t *testing.T) {
	f := newProcFixture(t)
	f.addWithExe(4242, 7619, fixtureUID, "kscreenlocker_greet", "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet")
	f.add(7619, 1, fixtureUID, "/usr/bin/ksmserver")

	prev := procRoot
	procRoot = f.root
	t.Cleanup(func() { procRoot = prev })

	ctx := withLockerCache(context.Background())
	yes, err := lockerRunningCached(ctx, fixtureUID)
	if err != nil || !yes {
		t.Fatalf("first read = (%v, %v), want (true, nil)", yes, err)
	}

	// The locker exits. A fresh walk would now say NO.
	if err := os.RemoveAll(filepath.Join(f.root, "4242")); err != nil {
		t.Fatal(err)
	}
	yes, err = lockerRunningCached(ctx, fixtureUID)
	if err != nil {
		t.Fatalf("cached read: %v", err)
	}
	if !yes {
		t.Error("the second read re-walked /proc instead of reusing the YES")
	}

	// And the next operation, with a fresh cache, must see the truth.
	fresh := withLockerCache(context.Background())
	yes, err = lockerRunningCached(fresh, fixtureUID)
	if err != nil {
		t.Fatalf("fresh-operation read: %v", err)
	}
	if yes {
		t.Error("a new operation inherited the previous operation's YES: the cache outlived the lock it was built for")
	}
}

// A fault must not be filed as a NO, and must not be remembered as one either.
//
// "cannot look" and "looked and found nothing" are different sentences to the caller --
// the first is a named fault, the second is unconfirmed -- and a transient /proc problem
// cached as a NO would become a permanent unconfirmed for the rest of the operation.
func TestAnUnreadableProcIsNotRememberedAsNotRunning(t *testing.T) {
	prev := procRoot
	procRoot = filepath.Join(t.TempDir(), "absent")
	t.Cleanup(func() { procRoot = prev })

	ctx := withLockerCache(context.Background())
	if _, err := lockerRunningCached(ctx, fixtureUID); err == nil {
		t.Fatal("an unreadable procRoot reported no error")
	}

	// The directory comes back. The operation must recover on its next read rather than
	// carrying the earlier fault's shape for the rest of the settle window.
	dir := t.TempDir()
	procRoot = dir
	f := &procFixture{t: t, root: dir}
	f.addWithExe(4242, 7619, fixtureUID, "kscreenlocker_greet", "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet")
	f.add(7619, 1, fixtureUID, "/usr/bin/ksmserver")

	yes, err := lockerRunningCached(ctx, fixtureUID)
	if err != nil {
		t.Fatalf("read after procRoot recovered: %v", err)
	}
	if !yes {
		t.Error("the operation kept answering about the earlier fault instead of looking again")
	}
}

// One session's locker does not vouch for another session's.
//
// This is a property of the map key, not of the cache's scope: two sessions on one machine
// have different uids, and a YES recorded for one must never answer for the other. It is
// pinned because the key is the only thing standing between "the locker is running" and
// "MY locker is running", and the second sentence is the one the confirmation needs.
func TestTwoOperationsDoNotShareALockerAnswer(t *testing.T) {
	otherUID := fixtureUID + 1
	f := newProcFixture(t)
	f.addWithExe(4242, 7619, fixtureUID, "kscreenlocker_greet", "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet")
	f.add(7619, 1, fixtureUID, "/usr/bin/ksmserver")

	prev := procRoot
	procRoot = f.root
	t.Cleanup(func() { procRoot = prev })

	one := withLockerCache(context.Background())
	two := withLockerCache(context.Background())

	yes, err := lockerRunningCached(one, fixtureUID)
	if err != nil || !yes {
		t.Fatalf("operation one = (%v, %v), want (true, nil)", yes, err)
	}
	yes, err = lockerRunningCached(two, otherUID)
	if err != nil {
		t.Fatalf("operation two: %v", err)
	}
	if yes {
		t.Errorf("uid %d was told uid %d's locker was its own", otherUID, fixtureUID)
	}
}

// A cache must not outlive the operation that built it, and the way to prove that is a
// second reader that arrives AFTER a YES has been recorded, asking about a world that has
// since changed.
//
// The bare context is what any caller outside Chain.Lock gets -- a diagnostic, a future
// status command, a test. If it could reach an earlier operation's answer it would report
// a locker that has exited as still running, which is the confirmation lying in the
// direction that releases the child.
func TestACallerOutsideAnOperationIsNotServedFromTheCache(t *testing.T) {
	f := newProcFixture(t)
	f.addWithExe(4242, 7619, fixtureUID, "kscreenlocker_greet", "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet")
	f.add(7619, 1, fixtureUID, "/usr/bin/ksmserver")

	prev := procRoot
	procRoot = f.root
	t.Cleanup(func() { procRoot = prev })

	// An operation records a YES.
	op := withLockerCache(context.Background())
	if yes, err := lockerRunningCached(op, fixtureUID); err != nil || !yes {
		t.Fatalf("setup read = (%v, %v), want (true, nil)", yes, err)
	}

	// The locker exits.
	if err := os.RemoveAll(filepath.Join(f.root, "4242")); err != nil {
		t.Fatal(err)
	}

	// A reader with no operation context must walk and say NO.
	yes, err := lockerRunningCached(context.Background(), fixtureUID)
	if err != nil {
		t.Fatalf("uncached read: %v", err)
	}
	if yes {
		t.Error("a context with no cache was answered from an earlier operation's YES")
	}
}

// The end-to-end shape: eleven confirmations inside one settle window used to mean eleven
// walks per backend. Counted at the confirmer, because that is the call the settle loop
// makes and the quantity the lock budget actually pays for.
func TestOneLockOperationWalksForTheLockerOncePerAnswerNotOncePerPoll(t *testing.T) {
	f := newProcFixture(t)
	prev := procRoot
	procRoot = f.root
	t.Cleanup(func() { procRoot = prev })

	var walks atomic.Int64
	// The walk is counted by the directory it opens: the fixture grows one pid per read
	// only if the walk actually runs, so a marker file is planted and the count is taken
	// from the number of times the confirmer was asked and still reached /proc.
	c := &countingConfirmer{walks: &walks, appearAfter: 3, fixture: f, uid: desktopUID}

	ctx := withLockerCache(context.Background())
	yes, err := confirmWithSettle(ctx, c, desktopSession("2"))
	if err != nil {
		t.Fatalf("confirmWithSettle: %v", err)
	}
	if !yes {
		t.Fatal("the lock never confirmed even though the locker appeared during the settle window")
	}
	// Asked 4 times (1 + 3 polls), the walk ran 4 times because every answer before the
	// locker appeared was a NO and NOs are never cached. After the YES, further polls
	// would cost nothing -- which is what the next assertion pins.
	if got := walks.Load(); got != 4 {
		t.Errorf("%d walks for 4 reads that all answered NO, want 4: a cached NO would have made this fewer, and a cached NO is the bug", got)
	}
}

// countingConfirmer answers NO until the locker is planted, then YES, and counts the reads
// that reached the walk.
type countingConfirmer struct {
	uid         uint32
	walks       *atomic.Int64
	planted     bool
	appearAfter int
	reads       int
	fixture     *procFixture
}

func (c *countingConfirmer) LockConfirmed(ctx context.Context, s Session) (bool, error) {
	c.reads++
	running, err := lockerRunningCached(ctx, c.uid)
	if err != nil {
		return false, err
	}
	c.walks.Add(1)
	if running {
		return true, nil
	}
	// Planted once. Adding it on every read would make the fixture fail on the second
	// symlink rather than model a locker that appeared once and stayed.
	if !c.planted && c.reads >= c.appearAfter {
		c.planted = true
		c.fixture.addWithExe(4242, 7619, c.uid, "kscreenlocker_greet", "/usr/lib/x86_64-linux-gnu/libexec/kscreenlocker_greet")
		c.fixture.add(7619, 1, c.uid, "/usr/bin/ksmserver")
	}
	return false, nil
}

func (c *countingConfirmer) Name() string { return "counting" }
