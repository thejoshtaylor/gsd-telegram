package handlers

import (
	"sync"
	"testing"
	"time"
)

// TestMediaGroupBuffer_SingleItem verifies that a single item fires the callback after timeout.
func TestMediaGroupBuffer_SingleItem(t *testing.T) {
	result := make(chan []string, 1)
	buf := NewMediaGroupBuffer(100*time.Millisecond, func(chatID, userID int64, paths []string, caption string) {
		result <- paths
	})

	buf.Add("g1", "/tmp/a.jpg", 123, 456, "")

	select {
	case paths := <-result:
		if len(paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(paths))
		}
		if paths[0] != "/tmp/a.jpg" {
			t.Errorf("paths[0] = %q, want %q", paths[0], "/tmp/a.jpg")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: callback never fired")
	}
}

// TestMediaGroupBuffer_MultipleItems verifies that 3 items added within the timeout
// window are batched into a single callback invocation.
func TestMediaGroupBuffer_MultipleItems(t *testing.T) {
	result := make(chan []string, 1)
	fireCount := 0
	var mu sync.Mutex
	buf := NewMediaGroupBuffer(200*time.Millisecond, func(chatID, userID int64, paths []string, caption string) {
		mu.Lock()
		fireCount++
		mu.Unlock()
		result <- paths
	})

	// Add 3 items within a short time window (well within the 200ms timeout).
	buf.Add("g1", "/tmp/1.jpg", 123, 456, "")
	buf.Add("g1", "/tmp/2.jpg", 123, 456, "")
	buf.Add("g1", "/tmp/3.jpg", 123, 456, "")

	select {
	case paths := <-result:
		if len(paths) != 3 {
			t.Fatalf("expected 3 paths, got %d: %v", len(paths), paths)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: callback never fired")
	}

	// Verify the callback fired exactly once.
	mu.Lock()
	count := fireCount
	mu.Unlock()
	if count != 1 {
		t.Errorf("callback fired %d times, want 1", count)
	}
}

// TestMediaGroupBuffer_IndependentGroups verifies that two different group IDs
// fire independently with their own paths.
func TestMediaGroupBuffer_IndependentGroups(t *testing.T) {
	type result struct {
		chatID int64
		paths  []string
	}
	results := make(chan result, 2)

	buf := NewMediaGroupBuffer(100*time.Millisecond, func(chatID, userID int64, paths []string, caption string) {
		results <- result{chatID: chatID, paths: paths}
	})

	buf.Add("g1", "/tmp/g1a.jpg", 111, 1, "")
	buf.Add("g2", "/tmp/g2a.jpg", 222, 2, "")

	got := make(map[int64][]string)
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			got[r.chatID] = r.paths
		case <-time.After(3 * time.Second):
			t.Fatal("timeout: not all groups fired")
		}
	}

	if paths, ok := got[111]; !ok || len(paths) != 1 || paths[0] != "/tmp/g1a.jpg" {
		t.Errorf("g1 paths = %v, want [/tmp/g1a.jpg]", got[111])
	}
	if paths, ok := got[222]; !ok || len(paths) != 1 || paths[0] != "/tmp/g2a.jpg" {
		t.Errorf("g2 paths = %v, want [/tmp/g2a.jpg]", got[222])
	}
}

// TestMediaGroupBuffer_FirstCaptionWins verifies that the first non-empty caption
// is preserved when multiple items have different captions.
func TestMediaGroupBuffer_FirstCaptionWins(t *testing.T) {
	type result struct {
		caption string
	}
	results := make(chan result, 1)

	buf := NewMediaGroupBuffer(100*time.Millisecond, func(chatID, userID int64, paths []string, caption string) {
		results <- result{caption: caption}
	})

	buf.Add("g1", "/tmp/a.jpg", 1, 1, "caption A")
	buf.Add("g1", "/tmp/b.jpg", 1, 1, "caption B")

	select {
	case r := <-results:
		if r.caption != "caption A" {
			t.Errorf("caption = %q, want %q (first caption should win)", r.caption, "caption A")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: callback never fired")
	}
}

// TestMediaGroupBuffer_EmptyCaptionSkipped verifies that an empty first caption does
// not block a non-empty caption from a subsequent item.
func TestMediaGroupBuffer_EmptyCaptionSkipped(t *testing.T) {
	type result struct {
		caption string
	}
	results := make(chan result, 1)

	buf := NewMediaGroupBuffer(100*time.Millisecond, func(chatID, userID int64, paths []string, caption string) {
		results <- result{caption: caption}
	})

	buf.Add("g1", "/tmp/a.jpg", 1, 1, "") // empty caption
	buf.Add("g1", "/tmp/b.jpg", 1, 1, "real caption")

	select {
	case r := <-results:
		if r.caption != "real caption" {
			t.Errorf("caption = %q, want %q (first non-empty caption should win)", r.caption, "real caption")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: callback never fired")
	}
}

// TestMediaGroupBuffer_ConcurrentAdd verifies that concurrent Add calls from multiple
// goroutines do not cause races or panics.
func TestMediaGroupBuffer_ConcurrentAdd(t *testing.T) {
	const numGoroutines = 10
	const numItems = 5
	done := make(chan struct{}, 1)

	buf := NewMediaGroupBuffer(200*time.Millisecond, func(chatID, userID int64, paths []string, caption string) {
		done <- struct{}{}
	})

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < numItems; j++ {
				buf.Add("group", "/tmp/file.jpg", 1, 1, "")
			}
		}(i)
	}
	wg.Wait()

	select {
	case <-done:
		// success — callback fired without panic
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: concurrent Add goroutines did not trigger callback")
	}
}
