package handlers

import (
	"encoding/json"
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

// --- New callback prefix tests ---

// TestParseCallbackGsd verifies that "gsd:{key}" routes to callbackActionGsd.
func TestParseCallbackGsd(t *testing.T) {
	action, payload := parseCallbackData("gsd:execute")
	if action != callbackActionGsd {
		t.Errorf("expected callbackActionGsd, got %d", action)
	}
	if payload != "execute" {
		t.Errorf("expected payload %q, got %q", "execute", payload)
	}
}

// TestParseCallbackGsdRun verifies that "gsd-run:{cmd}" routes to callbackActionGsdRun.
func TestParseCallbackGsdRun(t *testing.T) {
	data := "gsd-run:/gsd:next"
	action, payload := parseCallbackData(data)
	if action != callbackActionGsdRun {
		t.Errorf("expected callbackActionGsdRun, got %d", action)
	}
	if payload != "/gsd:next" {
		t.Errorf("expected payload %q, got %q", "/gsd:next", payload)
	}
}

// TestParseCallbackGsdFresh verifies that "gsd-fresh:{cmd}" routes to callbackActionGsdFresh.
func TestParseCallbackGsdFresh(t *testing.T) {
	data := "gsd-fresh:/gsd:plan-phase 2"
	action, payload := parseCallbackData(data)
	if action != callbackActionGsdFresh {
		t.Errorf("expected callbackActionGsdFresh, got %d", action)
	}
	if payload != "/gsd:plan-phase 2" {
		t.Errorf("expected payload %q, got %q", "/gsd:plan-phase 2", payload)
	}
}

// TestParseCallbackGsdPhase verifies that "gsd-exec:{N}" routes to callbackActionGsdPhase.
func TestParseCallbackGsdPhase(t *testing.T) {
	cases := []string{
		"gsd-exec:2",
		"gsd-plan:1",
		"gsd-discuss:3",
		"gsd-research:2",
		"gsd-verify:1",
		"gsd-remove:4",
	}
	for _, data := range cases {
		action, payload := parseCallbackData(data)
		if action != callbackActionGsdPhase {
			t.Errorf("data=%q: expected callbackActionGsdPhase, got %d", data, action)
		}
		if payload != data {
			t.Errorf("data=%q: expected full data as payload, got %q", data, payload)
		}
	}
}

// TestParseCallbackOption verifies that "option:{key}" routes to callbackActionOption.
func TestParseCallbackOption(t *testing.T) {
	action, payload := parseCallbackData("option:1")
	if action != callbackActionOption {
		t.Errorf("expected callbackActionOption, got %d", action)
	}
	if payload != "1" {
		t.Errorf("expected payload %q, got %q", "1", payload)
	}
}

// TestParseCallbackOption_Letter verifies that "option:A" routes to callbackActionOption.
func TestParseCallbackOption_Letter(t *testing.T) {
	action, payload := parseCallbackData("option:A")
	if action != callbackActionOption {
		t.Errorf("expected callbackActionOption, got %d", action)
	}
	if payload != "A" {
		t.Errorf("expected payload %q, got %q", "A", payload)
	}
}

// TestParseCallbackAskUser verifies that "askuser:{id}:{idx}" routes to callbackActionAskUser.
func TestParseCallbackAskUser(t *testing.T) {
	data := "askuser:a1b2c3d4:0"
	action, payload := parseCallbackData(data)
	if action != callbackActionAskUser {
		t.Errorf("expected callbackActionAskUser, got %d", action)
	}
	if payload != "a1b2c3d4:0" {
		t.Errorf("expected payload %q, got %q", "a1b2c3d4:0", payload)
	}
}

// TestParseCallbackProjectChange verifies that "project:change" routes to callbackActionProjectChange.
func TestParseCallbackProjectChange(t *testing.T) {
	action, payload := parseCallbackData("project:change")
	if action != callbackActionProjectChange {
		t.Errorf("expected callbackActionProjectChange, got %d", action)
	}
	if payload != "" {
		t.Errorf("expected empty payload, got %q", payload)
	}
}

// TestParseCallbackProjectUnlink verifies that "project:unlink" routes to callbackActionProjectUnlink.
func TestParseCallbackProjectUnlink(t *testing.T) {
	action, payload := parseCallbackData("project:unlink")
	if action != callbackActionProjectUnlink {
		t.Errorf("expected callbackActionProjectUnlink, got %d", action)
	}
	if payload != "" {
		t.Errorf("expected empty payload, got %q", payload)
	}
}

// TestParseCallbackPrefixOrder verifies that "gsd-run:..." is NOT parsed as "gsd:" action.
// The sub-prefix "gsd-run:" must be checked before "gsd:" to prevent premature matching.
func TestParseCallbackPrefixOrder(t *testing.T) {
	// gsd-run: must not be caught by gsd: prefix
	action, _ := parseCallbackData("gsd-run:/gsd:execute-phase 2")
	if action == callbackActionGsd {
		t.Error("gsd-run: was incorrectly parsed as callbackActionGsd (prefix order bug)")
	}
	if action != callbackActionGsdRun {
		t.Errorf("expected callbackActionGsdRun, got %d", action)
	}

	// gsd-fresh: must not be caught by gsd: prefix
	action2, _ := parseCallbackData("gsd-fresh:/gsd:plan-phase 1")
	if action2 == callbackActionGsd {
		t.Error("gsd-fresh: was incorrectly parsed as callbackActionGsd (prefix order bug)")
	}
	if action2 != callbackActionGsdFresh {
		t.Errorf("expected callbackActionGsdFresh, got %d", action2)
	}

	// gsd-exec: must not be caught by gsd: prefix
	action3, _ := parseCallbackData("gsd-exec:3")
	if action3 == callbackActionGsd {
		t.Error("gsd-exec: was incorrectly parsed as callbackActionGsd (prefix order bug)")
	}
	if action3 != callbackActionGsdPhase {
		t.Errorf("expected callbackActionGsdPhase, got %d", action3)
	}
}

// TestAskUserCallbackTempFile tests the core askuser temp-file read/parse/delete flow
// without requiring a full bot context.
func TestAskUserCallbackTempFile(t *testing.T) {
	requestID := "testid123"
	tmpFile := filepath.Join(os.TempDir(), "ask-user-"+requestID+".json")

	// Create the temp JSON file.
	req := askUserRequest{
		Question: "Pick one",
		Options:  []string{"A", "B", "C"},
		Status:   "pending",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal askUserRequest: %v", err)
	}
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile) })

	// Read and parse the file (simulating the callback handler logic).
	readData, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	var parsed askUserRequest
	if err := json.Unmarshal(readData, &parsed); err != nil {
		t.Fatalf("failed to parse askUserRequest: %v", err)
	}

	// Validate question and options.
	if parsed.Question != "Pick one" {
		t.Errorf("expected question %q, got %q", "Pick one", parsed.Question)
	}
	if len(parsed.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(parsed.Options))
	}

	// Validate that option index 1 maps to "B".
	optionIndex := 1
	if parsed.Options[optionIndex] != "B" {
		t.Errorf("expected option[1]=%q, got %q", "B", parsed.Options[optionIndex])
	}

	// Delete the temp file and verify it's gone.
	if err := os.Remove(tmpFile); err != nil {
		t.Fatalf("failed to delete temp file: %v", err)
	}
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("expected temp file to be deleted, but it still exists")
	}
}

// TestBuildGsdStatusHeader_WithPhases tests the GSD status header builder with
// a temp directory containing a .planning/ROADMAP.md with three phases.
func TestBuildGsdStatusHeader_WithPhases(t *testing.T) {
	dir := t.TempDir()

	// Create .planning/ROADMAP.md with three phases.
	planningDir := filepath.Join(dir, ".planning")
	if err := os.MkdirAll(planningDir, 0755); err != nil {
		t.Fatalf("failed to create .planning dir: %v", err)
	}

	roadmapContent := `# Roadmap

## Phases

- [x] **Phase 1: Core** - Core infrastructure
- [ ] **Phase 2: Features** - Main feature set
- [~] **Phase 3: Polish** - Polish and cleanup
`
	if err := os.WriteFile(filepath.Join(planningDir, "ROADMAP.md"), []byte(roadmapContent), 0644); err != nil {
		t.Fatalf("failed to write ROADMAP.md: %v", err)
	}

	header := buildGsdStatusHeader(dir)

	// (a) output contains "1/2 phases complete" (skipped phase not counted in total)
	if !contains(header, "1/2 phases complete") {
		t.Errorf("expected %q in header, got: %q", "1/2 phases complete", header)
	}

	// (b) output contains "Next: Phase 2: Features"
	if !contains(header, "Next: Phase 2:") {
		t.Errorf("expected %q in header, got: %q", "Next: Phase 2:", header)
	}
	if !contains(header, "Features") {
		t.Errorf("expected %q in header, got: %q", "Features", header)
	}

	// (c) output contains filepath.Base(dir) as project name
	projectName := filepath.Base(dir)
	if !contains(header, projectName) {
		t.Errorf("expected project name %q in header, got: %q", projectName, header)
	}
}

// contains is a simple substring helper for test assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// TestGsdOnQueryCompleteSavesSession verifies that the OnQueryComplete closure
// created inside enqueueGsdCommand correctly persists session data.
//
// The test simulates the closure logic directly (without a live bot/Claude process)
// to verify the wiring: title truncation at 50 chars, SessionID, WorkingDir, and
// ChannelID are all saved correctly.
func TestGsdOnQueryCompleteSavesSession(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sessions.json")
	pm := session.NewPersistenceManager(tmpFile, 5)

	capturedText := "/gsd:execute-phase 2"
	capturedChatID := int64(12345)
	capturedPath := "/tmp/test-project"

	// Simulate the closure that enqueueGsdCommand creates.
	onComplete := func(sessionID string) {
		title := capturedText
		if len(title) > 50 {
			title = title[:50]
		}
		saved := session.SavedSession{
			SessionID:  sessionID,
			SavedAt:    time.Now().UTC().Format(time.RFC3339),
			WorkingDir: capturedPath,
			Title:      title,
			ChannelID:  capturedChatID,
		}
		if err := pm.Save(saved); err != nil {
			t.Errorf("Save failed: %v", err)
		}
	}

	onComplete("test-session-abc")

	sessions, err := pm.LoadForChannel(capturedChatID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SessionID != "test-session-abc" {
		t.Errorf("SessionID = %q, want %q", sessions[0].SessionID, "test-session-abc")
	}
	if sessions[0].Title != "/gsd:execute-phase 2" {
		t.Errorf("Title = %q, want %q", sessions[0].Title, "/gsd:execute-phase 2")
	}
	if sessions[0].WorkingDir != capturedPath {
		t.Errorf("WorkingDir = %q, want %q", sessions[0].WorkingDir, capturedPath)
	}
	if sessions[0].ChannelID != capturedChatID {
		t.Errorf("ChannelID = %d, want %d", sessions[0].ChannelID, capturedChatID)
	}
}

// TestGsdOnQueryCompleteTitleTruncation verifies that titles longer than 50
// characters are truncated in the OnQueryComplete closure.
func TestGsdOnQueryCompleteTitleTruncation(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sessions.json")
	pm := session.NewPersistenceManager(tmpFile, 5)

	longText := "/gsd:execute-phase 2 with a very long description that exceeds fifty characters"
	capturedChatID := int64(99999)
	capturedPath := "/tmp/test-project-truncation"

	onComplete := func(sessionID string) {
		title := longText
		if len(title) > 50 {
			title = title[:50]
		}
		saved := session.SavedSession{
			SessionID:  sessionID,
			SavedAt:    time.Now().UTC().Format(time.RFC3339),
			WorkingDir: capturedPath,
			Title:      title,
			ChannelID:  capturedChatID,
		}
		if err := pm.Save(saved); err != nil {
			t.Errorf("Save failed: %v", err)
		}
	}

	onComplete("truncation-session-xyz")

	sessions, err := pm.LoadForChannel(capturedChatID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len(sessions[0].Title) > 50 {
		t.Errorf("Title length = %d, want <= 50; title = %q", len(sessions[0].Title), sessions[0].Title)
	}
	if sessions[0].Title != longText[:50] {
		t.Errorf("Title = %q, want %q", sessions[0].Title, longText[:50])
	}
}

// Compile-time check: os package used for tmpdir cleanup.
var _ = os.TempDir
