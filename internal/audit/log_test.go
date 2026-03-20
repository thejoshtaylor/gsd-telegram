package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"testing"
)

// TestAuditLogWrite verifies that Log writes a valid JSON entry containing required fields.
func TestAuditLogWrite(t *testing.T) {
	f, err := os.CreateTemp("", "audit-test-*.log")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	logger, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer logger.Close()

	evt := NewEvent("message", 12345, 67890)
	evt.Username = "testuser"
	evt.Message = "hello"

	if err := logger.Log(evt); err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Read the file back
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v — raw: %s", err, data)
	}

	if got["timestamp"] == nil {
		t.Error("expected timestamp field")
	}
	if got["action"] != "message" {
		t.Errorf("action = %v, want %q", got["action"], "message")
	}
	if got["user_id"] == nil {
		t.Error("expected user_id field")
	}
}

// TestAuditLogAppendOnly verifies that two events produce two valid JSON lines.
func TestAuditLogAppendOnly(t *testing.T) {
	f, err := os.CreateTemp("", "audit-test-*.log")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	logger, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for i := 0; i < 2; i++ {
		evt := NewEvent("message", int64(i), 999)
		if err := logger.Log(evt); err != nil {
			t.Fatalf("Log[%d]: %v", i, err)
		}
	}
	logger.Close()

	// Count lines
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(line, &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %s", lineCount+1, line)
		}
		lineCount++
	}

	if lineCount != 2 {
		t.Errorf("expected 2 JSON lines, got %d", lineCount)
	}
}

// TestAuditLogConcurrent verifies that concurrent logging does not cause data races or panics.
func TestAuditLogConcurrent(t *testing.T) {
	f, err := os.CreateTemp("", "audit-concurrent-*.log")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	logger, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer logger.Close()

	const goroutines = 10
	const eventsPerGoroutine = 10
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				evt := NewEvent("message", int64(g), int64(i))
				if err := logger.Log(evt); err != nil {
					t.Errorf("goroutine %d Log: %v", g, err)
				}
			}
		}(g)
	}
	wg.Wait()
	logger.Close()

	// Count lines
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(line, &obj); err != nil {
			t.Errorf("invalid JSON line: %s", line)
		}
		lineCount++
	}

	expected := goroutines * eventsPerGoroutine
	if lineCount != expected {
		t.Errorf("expected %d JSON lines, got %d", expected, lineCount)
	}
}
