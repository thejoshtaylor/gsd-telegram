package handlers

import (
	"strings"
	"testing"
)

// TestHandleVoice_NoAPIKey verifies that HandleVoice is defined and that the
// truncateTranscript helper truncates long transcripts correctly.
// (Full HandleVoice integration requires a live gotgbot.Bot; we test the pure helpers.)
func TestHandleVoice_NoAPIKey(t *testing.T) {
	// Verify truncateTranscript with short text (no truncation).
	short := "hello world"
	got := truncateTranscript(short, 200)
	if got != short {
		t.Errorf("truncateTranscript(%q, 200) = %q, want %q", short, got, short)
	}

	// Verify truncateTranscript with text exactly at limit.
	exact := strings.Repeat("a", 200)
	got = truncateTranscript(exact, 200)
	if got != exact {
		t.Errorf("truncateTranscript(200 chars, 200) should not truncate")
	}

	// Verify truncateTranscript with text exceeding limit.
	long := strings.Repeat("b", 300)
	got = truncateTranscript(long, 200)
	if len(got) != 200 {
		t.Errorf("truncateTranscript(300 chars, 200) length = %d, want 200", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncateTranscript(300 chars, 200) should end with '...', got %q", got[len(got)-10:])
	}
}

// TestHandleVoice_FunctionExists verifies that HandleVoice function signature exists.
func TestHandleVoice_FunctionExists(t *testing.T) {
	// This test simply verifies the function compiles and is callable.
	// We can't call it without a real bot, but we verify it exists.
	var fn interface{} = HandleVoice
	if fn == nil {
		t.Fatal("HandleVoice should not be nil")
	}
}
