package session

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/user/gsd-tele-go/internal/claude"
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

	msg := QueuedMessage{Text: "hello world", InstanceID: "inst-42"}
	ok := sess.Enqueue(msg)
	if !ok {
		t.Fatal("Enqueue returned false on empty queue")
	}

	select {
	case got := <-sess.queue:
		if got.Text != "hello world" {
			t.Errorf("expected text %q, got %q", "hello world", got.Text)
		}
		if got.InstanceID != "inst-42" {
			t.Errorf("expected InstanceID inst-42, got %q", got.InstanceID)
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

// writeNDJSON writes Claude NDJSON events to a temp file and returns the file path.
// The caller is responsible for removing the file.
func writeNDJSONTemp(t *testing.T, events []interface{}) string {
	t.Helper()
	f, err := os.CreateTemp("", "session-ndjson-*.jsonl")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	for _, ev := range events {
		b, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		f.Write(b)
		f.WriteString("\n")
	}
	f.Close()
	return f.Name()
}

// catBin returns the command and args that cat/type a file to stdout.
func catBin(filePath string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", []string{"/c", "type", filePath}
	}
	return "cat", []string{filePath}
}

// TestProcessMessageCapturesUsage verifies that after a successful processMessage,
// sess.LastUsage() returns the UsageData from the result event.
func TestProcessMessageCapturesUsage(t *testing.T) {
	usage := claude.UsageData{InputTokens: 500, OutputTokens: 200}
	evt := map[string]interface{}{
		"type":       "result",
		"session_id": "test-session-id",
		"usage": map[string]interface{}{
			"input_tokens":  500,
			"output_tokens": 200,
		},
	}
	tmpFile := writeNDJSONTemp(t, []interface{}{evt})
	defer os.Remove(tmpFile)

	bin, args := catBin(tmpFile)

	sess := NewSession(".")
	cfg := WorkerConfig{FilteredEnv: os.Environ(), testArgs: args}
	ctx := context.Background()
	msg := QueuedMessage{
		Text:       "",
		InstanceID: "inst-1",
		ErrCh:      make(chan error, 1),
		Callback: func(_ string) claude.StatusCallback {
			return func(_ claude.ClaudeEvent) error { return nil }
		},
	}

	sess.processMessage(ctx, bin, cfg, msg)

	// Drain ErrCh.
	select {
	case <-msg.ErrCh:
	case <-time.After(5 * time.Second):
		t.Fatal("processMessage timed out")
	}

	got := sess.LastUsage()
	if got == nil {
		t.Fatalf("LastUsage() = nil, want non-nil")
	}
	if got.InputTokens != usage.InputTokens {
		t.Errorf("LastUsage().InputTokens = %d, want %d", got.InputTokens, usage.InputTokens)
	}
	if got.OutputTokens != usage.OutputTokens {
		t.Errorf("LastUsage().OutputTokens = %d, want %d", got.OutputTokens, usage.OutputTokens)
	}
}

// TestProcessMessageCapturesContextPercent verifies that after a successful processMessage,
// sess.ContextPercent() returns the computed percentage from the result event.
func TestProcessMessageCapturesContextPercent(t *testing.T) {
	entry := map[string]interface{}{
		"inputTokens":   8000,
		"outputTokens":  2000,
		"contextWindow": 200000,
	}
	entryBytes, _ := json.Marshal(entry)
	evt := map[string]interface{}{
		"type":       "result",
		"session_id": "test-session-id",
		"modelUsage": map[string]json.RawMessage{
			"claude-sonnet": entryBytes,
		},
	}
	tmpFile := writeNDJSONTemp(t, []interface{}{evt})
	defer os.Remove(tmpFile)

	bin, args := catBin(tmpFile)

	sess := NewSession(".")
	cfg := WorkerConfig{FilteredEnv: os.Environ(), testArgs: args}
	ctx := context.Background()
	msg := QueuedMessage{
		Text:       "",
		InstanceID: "inst-1",
		ErrCh:      make(chan error, 1),
		Callback: func(_ string) claude.StatusCallback {
			return func(_ claude.ClaudeEvent) error { return nil }
		},
	}

	sess.processMessage(ctx, bin, cfg, msg)

	select {
	case <-msg.ErrCh:
	case <-time.After(5 * time.Second):
		t.Fatal("processMessage timed out")
	}

	pct := sess.ContextPercent()
	if pct == nil {
		t.Fatalf("ContextPercent() = nil, want non-nil")
	}
	// (8000 + 2000) * 100 / 200000 = 5
	if *pct != 5 {
		t.Errorf("ContextPercent() = %d, want 5", *pct)
	}
}

// TestProcessMessageNoUsageOnContextLimit verifies that after a context limit error,
// sess.LastUsage() and sess.ContextPercent() remain nil (not overwritten with partial data).
func TestProcessMessageNoUsageOnContextLimit(t *testing.T) {
	// Result event with context limit error message — triggers ErrContextLimit.
	evt := map[string]interface{}{
		"type":   "result",
		"result": "prompt too long for this request",
		"usage": map[string]interface{}{
			"input_tokens":  9999,
			"output_tokens": 1,
		},
	}
	tmpFile := writeNDJSONTemp(t, []interface{}{evt})
	defer os.Remove(tmpFile)

	bin, args := catBin(tmpFile)

	sess := NewSession(".")
	cfg := WorkerConfig{FilteredEnv: os.Environ(), testArgs: args}
	ctx := context.Background()
	msg := QueuedMessage{
		Text:       "",
		InstanceID: "inst-1",
		ErrCh:      make(chan error, 1),
		Callback: func(_ string) claude.StatusCallback {
			return func(_ claude.ClaudeEvent) error { return nil }
		},
	}

	sess.processMessage(ctx, bin, cfg, msg)

	select {
	case <-msg.ErrCh:
	case <-time.After(5 * time.Second):
		t.Fatal("processMessage timed out")
	}

	if got := sess.LastUsage(); got != nil {
		t.Errorf("LastUsage() = %+v after context limit, want nil", got)
	}
	if pct := sess.ContextPercent(); pct != nil {
		t.Errorf("ContextPercent() = %d after context limit, want nil", *pct)
	}
}
