package claude

import (
	"encoding/json"
	"testing"
)

// TestUnmarshalAssistantEvent verifies parsing of an "assistant" type NDJSON event.
func TestUnmarshalAssistantEvent(t *testing.T) {
	raw := `{
		"type": "assistant",
		"session_id": "sess-abc123",
		"message": {
			"id": "msg-001",
			"content": [
				{"type": "text", "text": "Hello, world!"}
			]
		}
	}`

	var evt ClaudeEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if evt.Type != "assistant" {
		t.Errorf("Type = %q, want %q", evt.Type, "assistant")
	}
	if evt.Message == nil {
		t.Fatal("Message should not be nil")
	}
	if len(evt.Message.Content) == 0 {
		t.Fatal("Content should not be empty")
	}
	if evt.Message.Content[0].Type != "text" {
		t.Errorf("Content[0].Type = %q, want %q", evt.Message.Content[0].Type, "text")
	}
}

// TestUnmarshalResultEvent verifies parsing of a "result" type event with usage data.
func TestUnmarshalResultEvent(t *testing.T) {
	raw := `{
		"type": "result",
		"session_id": "sess-xyz789",
		"result": "Task completed successfully.",
		"is_error": false,
		"usage": {
			"input_tokens": 1234,
			"output_tokens": 567,
			"cache_read_input_tokens": 100,
			"cache_creation_input_tokens": 50
		}
	}`

	var evt ClaudeEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if evt.IsError {
		t.Error("IsError should be false")
	}
	if evt.SessionID != "sess-xyz789" {
		t.Errorf("SessionID = %q, want %q", evt.SessionID, "sess-xyz789")
	}
	if evt.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if evt.Usage.InputTokens <= 0 {
		t.Errorf("Usage.InputTokens = %d, want > 0", evt.Usage.InputTokens)
	}
}

// TestUnmarshalToolUseEvent verifies parsing of a tool_use content block.
func TestUnmarshalToolUseEvent(t *testing.T) {
	raw := `{
		"type": "assistant",
		"session_id": "sess-tool",
		"message": {
			"id": "msg-tool-001",
			"content": [
				{
					"type": "tool_use",
					"id": "toolu_01",
					"name": "Read",
					"input": {"file_path": "/some/file.go"}
				}
			]
		}
	}`

	var evt ClaudeEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if evt.Message == nil || len(evt.Message.Content) == 0 {
		t.Fatal("expected content blocks")
	}

	block := evt.Message.Content[0]
	if block.Name != "Read" {
		t.Errorf("Name = %q, want %q", block.Name, "Read")
	}
	if block.Input["file_path"] == nil {
		t.Error("expected Input[\"file_path\"] to be set")
	}
}

// TestBuildArgsMinimal verifies BuildArgs output for the minimal (no session/model/paths) case.
func TestBuildArgsMinimal(t *testing.T) {
	args := BuildArgs("", nil, "", "")

	mustContain := []string{"-p", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"}
	for _, want := range mustContain {
		found := false
		for _, arg := range args {
			if arg == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("args should contain %q — got %v", want, args)
		}
	}

	// Should NOT contain --resume or --model when inputs are empty.
	for _, arg := range args {
		if arg == "--resume" || arg == "--model" {
			t.Errorf("unexpected arg %q in minimal build", arg)
		}
	}
}

// TestBuildArgsWithSession verifies --resume is added when a sessionID is provided.
func TestBuildArgsWithSession(t *testing.T) {
	args := BuildArgs("abc-123", nil, "", "")

	foundResume := false
	foundID := false
	for _, arg := range args {
		if arg == "--resume" {
			foundResume = true
		}
		if arg == "abc-123" {
			foundID = true
		}
	}
	if !foundResume {
		t.Error("args should contain --resume")
	}
	if !foundID {
		t.Error("args should contain session ID abc-123")
	}
}

// TestBuildArgsWithPaths verifies --add-dir is added with correct paths.
func TestBuildArgsWithPaths(t *testing.T) {
	args := BuildArgs("", []string{"/a", "/b"}, "", "")

	foundAddDir := false
	foundA := false
	foundB := false
	for _, arg := range args {
		switch arg {
		case "--add-dir":
			foundAddDir = true
		case "/a":
			foundA = true
		case "/b":
			foundB = true
		}
	}
	if !foundAddDir {
		t.Error("args should contain --add-dir")
	}
	if !foundA {
		t.Error("args should contain /a")
	}
	if !foundB {
		t.Error("args should contain /b")
	}
}

// TestIsContextLimitError verifies that each known pattern matches and normal text does not.
func TestIsContextLimitError(t *testing.T) {
	patterns := []string{
		"input length and max_tokens exceed context limit",
		"exceed context limit",
		"context limit exceeded",
		"prompt too long for this request",
		"conversation is too long",
		// Case-insensitive variants
		"EXCEED CONTEXT LIMIT",
		"Prompt Too Long",
	}
	for _, p := range patterns {
		if !isContextLimitError(p) {
			t.Errorf("isContextLimitError(%q) = false, want true", p)
		}
	}

	// Normal error messages should not match.
	nonPatterns := []string{
		"unexpected error occurred",
		"network timeout",
		"invalid JSON",
		"",
	}
	for _, p := range nonPatterns {
		if isContextLimitError(p) {
			t.Errorf("isContextLimitError(%q) = true, want false", p)
		}
	}
}

// TestContextPercent verifies context percentage calculation from ModelUsage.
func TestContextPercent(t *testing.T) {
	// 80000 input + 4000 output = 84000 tokens; 84000 * 100 / 200000 = 42
	raw := `{
		"type": "result",
		"modelUsage": {
			"claude-3-5-sonnet": {
				"inputTokens": 80000,
				"outputTokens": 4000,
				"cacheReadInputTokens": 0,
				"cacheCreationInputTokens": 0,
				"contextWindow": 200000
			}
		}
	}`

	var evt ClaudeEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	pct := evt.ContextPercent()
	if pct == nil {
		t.Fatal("ContextPercent() should not return nil")
	}
	if *pct != 42 {
		t.Errorf("ContextPercent() = %d, want 42", *pct)
	}
}

// TestContextPercentEmptyModelUsage verifies nil is returned when ModelUsage is empty.
func TestContextPercentEmptyModelUsage(t *testing.T) {
	evt := ClaudeEvent{Type: "result"}
	if pct := evt.ContextPercent(); pct != nil {
		t.Errorf("ContextPercent() = %d on empty ModelUsage, want nil", *pct)
	}
}
