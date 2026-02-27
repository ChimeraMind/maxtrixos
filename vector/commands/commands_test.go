package commands

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func TestSignalGuardPushCleanupLIFO(t *testing.T) {
	var sg SignalGuard

	var order []int
	sg.PushCleanup(func() { order = append(order, 1) })
	sg.PushCleanup(func() { order = append(order, 2) })
	sg.PushCleanup(func() { order = append(order, 3) })

	sg.RunCleanups()

	if len(order) != 3 {
		t.Fatalf("expected 3 cleanups, got %d", len(order))
	}
	// Last pushed should run first (LIFO).
	want := []int{3, 2, 1}
	for i, v := range want {
		if order[i] != v {
			t.Errorf("cleanup[%d] = %d, want %d", i, order[i], v)
		}
	}
}

func TestSignalGuardRunCleanupsIdempotent(t *testing.T) {
	var sg SignalGuard
	var count int
	sg.PushCleanup(func() { count++ })

	sg.RunCleanups()
	sg.RunCleanups() // second call should be a no-op

	if count != 1 {
		t.Errorf("cleanup ran %d times, want 1", count)
	}
}

func TestSignalGuardPanicInCleanupDoesNotAbort(t *testing.T) {
	var sg SignalGuard

	var ran bool
	sg.PushCleanup(func() { ran = true }) // pushed first → runs last in LIFO
	sg.PushCleanup(func() { panic("boom") })

	sg.RunCleanups()

	if !ran {
		t.Error("cleanup after panicking cleanup did not run")
	}
}

func TestSignalGuardArmDisarm(t *testing.T) {
	var sg SignalGuard

	sg.Arm()
	if !sg.armed {
		t.Fatal("expected armed after Arm()")
	}

	// Arm again should be a no-op.
	sg.Arm()
	if !sg.armed {
		t.Fatal("expected still armed after double Arm()")
	}

	sg.Disarm()
	if sg.armed {
		t.Fatal("expected disarmed after Disarm()")
	}

	// Disarm again should be a no-op.
	sg.Disarm()
	if sg.armed {
		t.Fatal("expected still disarmed after double Disarm()")
	}
}

func TestSignalGuardDisarmClearsStack(t *testing.T) {
	var sg SignalGuard
	sg.Arm()

	var count int
	sg.PushCleanup(func() { count++ })
	sg.Disarm()

	sg.RunCleanups()
	if count != 0 {
		t.Error("cleanups ran after Disarm() cleared stack")
	}
}

func TestRunWithGuardNormalExecution(t *testing.T) {
	var sg SignalGuard
	var cleaned int32

	err := sg.RunWithGuard(func() error {
		sg.PushCleanup(func() { atomic.AddInt32(&cleaned, 1) })
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&cleaned) != 1 {
		t.Error("cleanup did not run after normal return")
	}
	if sg.armed {
		t.Error("guard still armed after RunWithGuard completed")
	}
}

func TestRunWithGuardOnError(t *testing.T) {
	var sg SignalGuard
	var cleaned int32

	err := sg.RunWithGuard(func() error {
		sg.PushCleanup(func() { atomic.AddInt32(&cleaned, 1) })
		return fmt.Errorf("something went wrong")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if atomic.LoadInt32(&cleaned) != 1 {
		t.Error("cleanup did not run after error return")
	}
}

func TestRunWithGuardOnPanic(t *testing.T) {
	var sg SignalGuard
	var cleaned int32

	err := sg.RunWithGuard(func() error {
		sg.PushCleanup(func() { atomic.AddInt32(&cleaned, 1) })
		panic("kaboom")
	})

	if err == nil {
		t.Fatal("expected error from recovered panic, got nil")
	}
	if atomic.LoadInt32(&cleaned) != 1 {
		t.Error("cleanup did not run after panic")
	}
	if sg.armed {
		t.Error("guard still armed after RunWithGuard recovered panic")
	}
}
