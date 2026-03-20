package session

import (
	"testing"
	"time"
)

// TestNewSession verifies that a freshly created Session has the expected initial state.
func TestNewSession(t *testing.T) {
	sess := NewSession("/tmp/test")

	if sess.state != StateIdle {
		t.Errorf("expected StateIdle, got %d", sess.state)
	}

	if cap(sess.queue) != 5 {
		t.Errorf("expected queue capacity 5, got %d", cap(sess.queue))
	}

	if sess.workingDir != "/tmp/test" {
		t.Errorf("expected workingDir /tmp/test, got %s", sess.workingDir)
	}

	if sess.startedAt.IsZero() {
		t.Error("startedAt should not be zero")
	}
}

// TestEnqueueAndDequeue verifies that Enqueue adds a message that can be read from the queue.
func TestEnqueueAndDequeue(t *testing.T) {
	sess := NewSession("/tmp/test")

	msg := QueuedMessage{Text: "hello world", ChatID: 42, UserID: 7}
	ok := sess.Enqueue(msg)
	if !ok {
		t.Fatal("Enqueue returned false on empty queue")
	}

	select {
	case got := <-sess.queue:
		if got.Text != "hello world" {
			t.Errorf("expected text %q, got %q", "hello world", got.Text)
		}
		if got.ChatID != 42 {
			t.Errorf("expected ChatID 42, got %d", got.ChatID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out reading from queue")
	}
}

// TestEnqueueFull verifies that Enqueue returns false when the queue is at capacity.
func TestEnqueueFull(t *testing.T) {
	sess := NewSession("/tmp/test")

	// Fill to capacity (5).
	for i := 0; i < 5; i++ {
		ok := sess.Enqueue(QueuedMessage{Text: "msg"})
		if !ok {
			t.Fatalf("Enqueue returned false at index %d (queue not yet full)", i)
		}
	}

	// Next enqueue should fail.
	ok := sess.Enqueue(QueuedMessage{Text: "overflow"})
	if ok {
		t.Error("expected Enqueue to return false on full queue, got true")
	}
}

// TestStopSession verifies that Stop() sends to stopCh.
func TestStopSession(t *testing.T) {
	sess := NewSession("/tmp/test")

	// Stop should signal stopCh.
	sess.Stop()

	select {
	case <-sess.stopCh:
		// expected
	case <-time.After(time.Second):
		t.Fatal("Stop() did not signal stopCh within 1s")
	}
}

// TestStopSessionNonBlocking verifies that Stop() does not block if stopCh is already signaled.
func TestStopSessionNonBlocking(t *testing.T) {
	sess := NewSession("/tmp/test")

	done := make(chan struct{})
	go func() {
		// Call Stop twice — second call must not block.
		sess.Stop()
		sess.Stop()
		close(done)
	}()

	select {
	case <-done:
		// expected
	case <-time.After(time.Second):
		t.Fatal("Stop() blocked on second call")
	}
}

// TestSessionIDAccessors tests SetSessionID and SessionID under lock.
func TestSessionIDAccessors(t *testing.T) {
	sess := NewSession("/tmp")

	if id := sess.SessionID(); id != "" {
		t.Errorf("expected empty sessionID, got %q", id)
	}

	sess.SetSessionID("abc-123")

	if id := sess.SessionID(); id != "abc-123" {
		t.Errorf("expected abc-123, got %q", id)
	}
}

// TestMarkInterrupt verifies that MarkInterrupt sets the interruptPending flag.
func TestMarkInterrupt(t *testing.T) {
	sess := NewSession("/tmp")
	sess.MarkInterrupt()

	sess.mu.Lock()
	pending := sess.interruptPending
	sess.mu.Unlock()

	if !pending {
		t.Error("expected interruptPending to be true after MarkInterrupt")
	}
}
