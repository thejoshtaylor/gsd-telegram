package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/gsd-tele-go/internal/session"
)

// TestCallbackRouteResume verifies that "resume:<uuid>" is parsed to callbackActionResume.
func TestCallbackRouteResume(t *testing.T) {
	data := "resume:abc12345-def6-7890-ghij-klmnopqrstuv"
	action, payload := parseCallbackData(data)

	if action != callbackActionResume {
		t.Errorf("expected callbackActionResume, got %d", action)
	}
	if payload != "abc12345-def6-7890-ghij-klmnopqrstuv" {
		t.Errorf("expected full UUID payload, got %q", payload)
	}
}

// TestCallbackRouteActionStop verifies that "action:stop" routes to callbackActionStop.
func TestCallbackRouteActionStop(t *testing.T) {
	action, payload := parseCallbackData("action:stop")

	if action != callbackActionStop {
		t.Errorf("expected callbackActionStop, got %d", action)
	}
	if payload != "" {
		t.Errorf("expected empty payload for stop, got %q", payload)
	}
}

// TestCallbackRouteActionNew verifies that "action:new" routes to callbackActionNew.
func TestCallbackRouteActionNew(t *testing.T) {
	action, payload := parseCallbackData("action:new")

	if action != callbackActionNew {
		t.Errorf("expected callbackActionNew, got %d", action)
	}
	if payload != "" {
		t.Errorf("expected empty payload for new, got %q", payload)
	}
}

// TestCallbackRouteActionRetry verifies that "action:retry" routes to callbackActionRetry.
func TestCallbackRouteActionRetry(t *testing.T) {
	action, _ := parseCallbackData("action:retry")
	if action != callbackActionRetry {
		t.Errorf("expected callbackActionRetry, got %d", action)
	}
}

// TestCallbackRouteUnknown verifies that unknown data routes to callbackActionUnknown.
func TestCallbackRouteUnknown(t *testing.T) {
	cases := []string{
		"unknown:xyz",
		"",
		"action:",
		"resume",
		"RESUME:abc",
	}
	for _, data := range cases {
		action, _ := parseCallbackData(data)
		if action != callbackActionUnknown {
			t.Errorf("data=%q: expected callbackActionUnknown, got %d", data, action)
		}
	}
}

// TestResumeRestoresSessionID tests the end-to-end resume flow:
// save a session to persistence, load via parseCallbackData, apply to session store,
// verify session.SessionID() returns the restored ID.
func TestResumeRestoresSessionID(t *testing.T) {
	// Set up temporary persistence file.
	dir := t.TempDir()
	persistPath := filepath.Join(dir, "session-history.json")
	pm := session.NewPersistenceManager(persistPath, 5)

	const channelID int64 = 99001
	const sessionID = "abc12345-def6-7890-abcd-ef0123456789"

	// Save a session to persistence.
	saved := session.SavedSession{
		SessionID:  sessionID,
		SavedAt:    time.Now().UTC().Format(time.RFC3339),
		WorkingDir: dir,
		Title:      "Test session for resume",
		ChannelID:  channelID,
	}
	if err := pm.Save(saved); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	// Load it back to verify it persisted.
	sessions, err := pm.LoadForChannel(channelID)
	if err != nil {
		t.Fatalf("failed to load sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 saved session, got %d", len(sessions))
	}
	if sessions[0].SessionID != sessionID {
		t.Errorf("expected session ID %q, got %q", sessionID, sessions[0].SessionID)
	}

	// Simulate the callback routing: parse the callback data.
	callbackData := "resume:" + sessionID
	action, payload := parseCallbackData(callbackData)
	if action != callbackActionResume {
		t.Fatalf("expected callbackActionResume, got %d", action)
	}
	if payload != sessionID {
		t.Fatalf("expected payload %q, got %q", sessionID, payload)
	}

	// Apply the restored session ID to the session store.
	store := session.NewSessionStore()
	sess := store.GetOrCreate(channelID, dir)
	sess.SetSessionID(payload)

	// Verify that the session now has the restored ID.
	if got := sess.SessionID(); got != sessionID {
		t.Errorf("expected SessionID %q after restore, got %q", sessionID, got)
	}
}

// TestCallbackParseResumePrefixStripped verifies that the "resume:" prefix is stripped from the payload.
func TestCallbackParseResumePrefixStripped(t *testing.T) {
	sessionID := "ffffffff-aaaa-bbbb-cccc-000000000001"
	action, payload := parseCallbackData("resume:" + sessionID)

	if action != callbackActionResume {
		t.Errorf("expected callbackActionResume")
	}
	if payload != sessionID {
		t.Errorf("expected raw session ID %q in payload, got %q", sessionID, payload)
	}
}

// TestCallbackParseAllActions validates all known action strings.
func TestCallbackParseAllActions(t *testing.T) {
	cases := []struct {
		data     string
		expected callbackAction
	}{
		{"action:stop", callbackActionStop},
		{"action:new", callbackActionNew},
		{"action:retry", callbackActionRetry},
	}
	for _, tc := range cases {
		action, _ := parseCallbackData(tc.data)
		if action != tc.expected {
			t.Errorf("data=%q: expected action %d, got %d", tc.data, tc.expected, action)
		}
	}
}

// Compile-time check: os package used for tmpdir cleanup.
var _ = os.TempDir
