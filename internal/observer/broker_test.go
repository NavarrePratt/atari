package observer

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewSessionBroker(t *testing.T) {
	b := NewSessionBroker()
	if b == nil {
		t.Fatal("expected non-nil broker")
	}
	if b.Holder() != "" {
		t.Errorf("expected empty holder, got %q", b.Holder())
	}
	if b.IsHeld() {
		t.Error("expected broker to not be held initially")
	}
}

func TestSessionBroker_AcquireRelease(t *testing.T) {
	b := NewSessionBroker()
	ctx := context.Background()

	// Acquire should succeed
	err := b.Acquire(ctx, "drain", 5*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Check holder
	if b.Holder() != "drain" {
		t.Errorf("expected holder %q, got %q", "drain", b.Holder())
	}
	if !b.IsHeld() {
		t.Error("expected broker to be held")
	}

	// Release
	b.Release()
	if b.Holder() != "" {
		t.Errorf("expected empty holder after release, got %q", b.Holder())
	}
	if b.IsHeld() {
		t.Error("expected broker to not be held after release")
	}
}

func TestSessionBroker_AcquireEmptyHolder(t *testing.T) {
	b := NewSessionBroker()
	ctx := context.Background()

	err := b.Acquire(ctx, "", 5*time.Second)
	if err == nil {
		t.Error("expected error for empty holder")
	}
}

func TestSessionBroker_DoubleRelease(t *testing.T) {
	b := NewSessionBroker()
	ctx := context.Background()

	err := b.Acquire(ctx, "drain", 5*time.Second)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Double release should be safe
	b.Release()
	b.Release() // Should not panic or block

	// Should be able to acquire again
	err = b.Acquire(ctx, "observer", 5*time.Second)
	if err != nil {
		t.Fatalf("Acquire after double release failed: %v", err)
	}
	b.Release()
}

func TestSessionBroker_Timeout(t *testing.T) {
	b := NewSessionBroker()
	ctx := context.Background()

	// First acquire succeeds
	err := b.Acquire(ctx, "drain", 5*time.Second)
	if err != nil {
		t.Fatalf("First Acquire failed: %v", err)
	}

	// Second acquire should timeout
	start := time.Now()
	err = b.Acquire(ctx, "observer", 100*time.Millisecond)
	elapsed := time.Since(start)

	if err != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("timeout happened too quickly: %v", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("timeout took too long: %v", elapsed)
	}

	b.Release()
}

func TestSessionBroker_ContextCancellation(t *testing.T) {
	b := NewSessionBroker()

	// First acquire succeeds
	err := b.Acquire(context.Background(), "drain", 5*time.Second)
	if err != nil {
		t.Fatalf("First Acquire failed: %v", err)
	}

	// Second acquire with cancelled context
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err = b.Acquire(ctx, "observer", 5*time.Second)
	elapsed := time.Since(start)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("cancellation happened too quickly: %v", elapsed)
	}
	if elapsed > 150*time.Millisecond {
		t.Errorf("cancellation took too long: %v", elapsed)
	}

	b.Release()
}

func TestSessionBroker_TryAcquire(t *testing.T) {
	b := NewSessionBroker()

	// TryAcquire should succeed when unlocked
	if !b.TryAcquire("drain") {
		t.Error("TryAcquire should succeed when unlocked")
	}
	if b.Holder() != "drain" {
		t.Errorf("expected holder %q, got %q", "drain", b.Holder())
	}

	// TryAcquire should fail when locked
	if b.TryAcquire("observer") {
		t.Error("TryAcquire should fail when locked")
	}
	// Holder should still be drain
	if b.Holder() != "drain" {
		t.Errorf("expected holder to remain %q, got %q", "drain", b.Holder())
	}

	b.Release()

	// TryAcquire should succeed again after release
	if !b.TryAcquire("observer") {
		t.Error("TryAcquire should succeed after release")
	}
	if b.Holder() != "observer" {
		t.Errorf("expected holder %q, got %q", "observer", b.Holder())
	}
	b.Release()
}

func TestSessionBroker_TryAcquireEmptyHolder(t *testing.T) {
	b := NewSessionBroker()

	if b.TryAcquire("") {
		t.Error("TryAcquire should fail for empty holder")
	}
}

func TestSessionBroker_ConcurrentAcquire(t *testing.T) {
	b := NewSessionBroker()
	ctx := context.Background()

	const numGoroutines = 10
	var successCount atomic.Int32
	var wg sync.WaitGroup

	// Start multiple goroutines trying to acquire
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if b.TryAcquire("worker") {
				successCount.Add(1)
				// Hold for a bit then release
				time.Sleep(10 * time.Millisecond)
				b.Release()
			}
		}(i)
	}

	wg.Wait()

	// At least one should have succeeded
	if successCount.Load() < 1 {
		t.Error("expected at least one goroutine to acquire the lock")
	}

	// Should be unlocked now
	if b.IsHeld() {
		t.Error("expected broker to be unlocked after all goroutines complete")
	}

	// Should be able to acquire again
	err := b.Acquire(ctx, "final", 5*time.Second)
	if err != nil {
		t.Fatalf("Final Acquire failed: %v", err)
	}
	b.Release()
}

func TestSessionBroker_ConcurrentAcquireBlocking(t *testing.T) {
	b := NewSessionBroker()
	ctx := context.Background()

	// First goroutine acquires and holds
	err := b.Acquire(ctx, "first", 5*time.Second)
	if err != nil {
		t.Fatalf("First Acquire failed: %v", err)
	}

	const numWaiters = 5
	var completedCount atomic.Int32
	var wg sync.WaitGroup

	// Start waiters that will block
	for i := 0; i < numWaiters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := b.Acquire(ctx, "waiter", 2*time.Second)
			if err == nil {
				completedCount.Add(1)
				b.Release()
			}
		}(i)
	}

	// Give waiters time to start blocking
	time.Sleep(50 * time.Millisecond)

	// Release the first holder
	b.Release()

	// Wait for all waiters to complete
	wg.Wait()

	// Each waiter should acquire and release in sequence
	// At least some should succeed (the ones that didn't timeout)
	if completedCount.Load() < 1 {
		t.Error("expected at least one waiter to complete")
	}
}

func TestSessionBroker_HolderTransition(t *testing.T) {
	b := NewSessionBroker()
	ctx := context.Background()

	// Simulate drain -> observer -> drain transition
	err := b.Acquire(ctx, "drain", 5*time.Second)
	if err != nil {
		t.Fatalf("Drain Acquire failed: %v", err)
	}
	if b.Holder() != "drain" {
		t.Errorf("expected holder %q, got %q", "drain", b.Holder())
	}

	b.Release()
	if b.Holder() != "" {
		t.Errorf("expected empty holder, got %q", b.Holder())
	}

	err = b.Acquire(ctx, "observer", 5*time.Second)
	if err != nil {
		t.Fatalf("Observer Acquire failed: %v", err)
	}
	if b.Holder() != "observer" {
		t.Errorf("expected holder %q, got %q", "observer", b.Holder())
	}

	b.Release()

	err = b.Acquire(ctx, "drain", 5*time.Second)
	if err != nil {
		t.Fatalf("Second Drain Acquire failed: %v", err)
	}
	if b.Holder() != "drain" {
		t.Errorf("expected holder %q, got %q", "drain", b.Holder())
	}
	b.Release()
}

func TestSessionBroker_ContextDeadline(t *testing.T) {
	b := NewSessionBroker()

	// First acquire succeeds
	err := b.Acquire(context.Background(), "drain", 5*time.Second)
	if err != nil {
		t.Fatalf("First Acquire failed: %v", err)
	}

	// Second acquire with deadline context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = b.Acquire(ctx, "observer", 5*time.Second)

	// Should fail with context deadline
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	b.Release()
}
