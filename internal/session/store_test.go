package session

import (
	"sync"
	"testing"
)

// TestGetOrCreateNew verifies that GetOrCreate returns a fresh session for a new channel ID.
func TestGetOrCreateNew(t *testing.T) {
	store := NewSessionStore()
	sess := store.GetOrCreate(1001, "/tmp/project")

	if sess == nil {
		t.Fatal("expected non-nil session")
	}
	if sess.WorkingDir() != "/tmp/project" {
		t.Errorf("expected workingDir /tmp/project, got %s", sess.WorkingDir())
	}
}

// TestGetOrCreateExisting verifies that calling GetOrCreate twice returns the same pointer.
func TestGetOrCreateExisting(t *testing.T) {
	store := NewSessionStore()
	first := store.GetOrCreate(1002, "/tmp/a")
	second := store.GetOrCreate(1002, "/tmp/b") // workingDir arg ignored on second call

	if first != second {
		t.Error("expected same *Session pointer for same channelID")
	}
	// Working dir should be from the first creation, not the second.
	if second.WorkingDir() != "/tmp/a" {
		t.Errorf("expected workingDir /tmp/a, got %s", second.WorkingDir())
	}
}

// TestGetNonExistent verifies that Get returns (nil, false) for an unknown channel ID.
func TestGetNonExistent(t *testing.T) {
	store := NewSessionStore()
	sess, ok := store.Get(9999)
	if ok {
		t.Error("expected ok=false for unknown channel")
	}
	if sess != nil {
		t.Error("expected nil session for unknown channel")
	}
}

// TestConcurrentGetOrCreate verifies that 20 goroutines calling GetOrCreate simultaneously
// all receive the same *Session pointer and the race detector reports no issues.
func TestConcurrentGetOrCreate(t *testing.T) {
	store := NewSessionStore()
	const goroutines = 20

	results := make([]*Session, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i] = store.GetOrCreate(5000, "/tmp/concurrent")
		}()
	}
	wg.Wait()

	// All results should be the same pointer.
	first := results[0]
	for i, sess := range results {
		if sess != first {
			t.Errorf("goroutine %d got different *Session pointer", i)
		}
	}
}

// TestRemove verifies that after Remove, Get returns (nil, false).
func TestRemove(t *testing.T) {
	store := NewSessionStore()
	store.GetOrCreate(2001, "/tmp/remove")

	store.Remove(2001)

	sess, ok := store.Get(2001)
	if ok {
		t.Error("expected ok=false after Remove")
	}
	if sess != nil {
		t.Error("expected nil session after Remove")
	}
}

// TestCount verifies that Count reflects the number of active sessions.
func TestCount(t *testing.T) {
	store := NewSessionStore()

	if n := store.Count(); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}

	store.GetOrCreate(3001, "/a")
	store.GetOrCreate(3002, "/b")

	if n := store.Count(); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}

	store.Remove(3001)

	if n := store.Count(); n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

// TestAll verifies that All returns a copy of the sessions map.
func TestAll(t *testing.T) {
	store := NewSessionStore()
	store.GetOrCreate(4001, "/x")
	store.GetOrCreate(4002, "/y")

	all := store.All()
	if len(all) != 2 {
		t.Errorf("expected 2 sessions in All(), got %d", len(all))
	}

	// Modifying the returned map must not affect the store.
	delete(all, 4001)
	if store.Count() != 2 {
		t.Error("modifying All() result changed store count")
	}
}
