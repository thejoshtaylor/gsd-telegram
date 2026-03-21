package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeTempJSON writes data as JSON to a temp file and returns the path.
func writeTempJSON(t *testing.T, dir, name string, data interface{}) string {
	t.Helper()
	path := filepath.Join(dir, name)
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
	return path
}

// readSessionHistoryFile reads and parses a sessions JSON file.
func readSessionHistoryFile(t *testing.T, path string) SessionHistory {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	var h SessionHistory
	if err := json.Unmarshal(data, &h); err != nil {
		t.Fatalf("json.Unmarshal sessions file: %v", err)
	}
	return h
}

// TestMigrateEmptyMappings verifies that with no mappings, all sessions are logged
// as unmappable and the output file has an empty sessions array.
func TestMigrateEmptyMappings(t *testing.T) {
	dir := t.TempDir()

	// Old sessions.json: 2 sessions, both with channel IDs.
	oldSessions := map[string]interface{}{
		"sessions": []map[string]interface{}{
			{
				"session_id":  "sess-aaa",
				"saved_at":    "2026-01-01T10:00:00Z",
				"working_dir": "/proj/a",
				"title":       "Session A",
				"channel_id":  123456789,
			},
			{
				"session_id":  "sess-bbb",
				"saved_at":    "2026-01-02T10:00:00Z",
				"working_dir": "/proj/b",
				"title":       "Session B",
				"channel_id":  987654321,
			},
		},
	}
	sessPath := writeTempJSON(t, dir, "sessions.json", oldSessions)

	// Empty mappings file.
	emptyMappings := map[string]interface{}{
		"mappings": map[string]interface{}{},
	}
	mappingsPath := writeTempJSON(t, dir, "mappings.json", emptyMappings)

	result, err := MigrateSessionHistory(sessPath, mappingsPath)
	if err != nil {
		t.Fatalf("MigrateSessionHistory: %v", err)
	}

	if result.Migrated != 0 {
		t.Errorf("Migrated: want 0, got %d", result.Migrated)
	}
	if result.Unmapped != 2 {
		t.Errorf("Unmapped: want 2, got %d", result.Unmapped)
	}
	if len(result.UnmappedEntries) != 2 {
		t.Errorf("UnmappedEntries: want 2, got %d", len(result.UnmappedEntries))
	}

	// Output file should have empty sessions array.
	h := readSessionHistoryFile(t, sessPath)
	if len(h.Sessions) != 0 {
		t.Errorf("expected 0 sessions in output, got %d", len(h.Sessions))
	}
}

// TestMigrateMatchingMappings verifies that channel IDs with matching mappings
// are converted to project-name instance IDs.
func TestMigrateMatchingMappings(t *testing.T) {
	dir := t.TempDir()

	// Old sessions.json: 2 sessions matching 2 different channels.
	oldSessions := map[string]interface{}{
		"sessions": []map[string]interface{}{
			{
				"session_id":  "sess-111",
				"saved_at":    "2026-02-01T10:00:00Z",
				"working_dir": "/proj/alpha",
				"title":       "Alpha session",
				"channel_id":  111111,
			},
			{
				"session_id":  "sess-222",
				"saved_at":    "2026-02-02T10:00:00Z",
				"working_dir": "/proj/beta",
				"title":       "Beta session",
				"channel_id":  222222,
			},
		},
	}
	sessPath := writeTempJSON(t, dir, "sessions.json", oldSessions)

	// Mappings file: both channel IDs mapped to project names.
	mappings := map[string]interface{}{
		"mappings": map[string]interface{}{
			"111111": map[string]interface{}{
				"path":      "/proj/alpha",
				"name":      "my-alpha",
				"linked_at": "2026-01-15T10:00:00Z",
			},
			"222222": map[string]interface{}{
				"path":      "/proj/beta",
				"name":      "my-beta",
				"linked_at": "2026-01-16T10:00:00Z",
			},
		},
	}
	mappingsPath := writeTempJSON(t, dir, "mappings.json", mappings)

	result, err := MigrateSessionHistory(sessPath, mappingsPath)
	if err != nil {
		t.Fatalf("MigrateSessionHistory: %v", err)
	}

	if result.Migrated != 2 {
		t.Errorf("Migrated: want 2, got %d", result.Migrated)
	}
	if result.Unmapped != 0 {
		t.Errorf("Unmapped: want 0, got %d", result.Unmapped)
	}

	// Output file should have 2 sessions with instance_id set to project name.
	h := readSessionHistoryFile(t, sessPath)
	if len(h.Sessions) != 2 {
		t.Fatalf("expected 2 sessions in output, got %d", len(h.Sessions))
	}

	// Build lookup of session ID -> InstanceID for verification.
	lookup := make(map[string]string)
	for _, s := range h.Sessions {
		lookup[s.SessionID] = s.InstanceID
	}

	if lookup["sess-111"] != "my-alpha" {
		t.Errorf("sess-111: want InstanceID=my-alpha, got %q", lookup["sess-111"])
	}
	if lookup["sess-222"] != "my-beta" {
		t.Errorf("sess-222: want InstanceID=my-beta, got %q", lookup["sess-222"])
	}
}

// TestMigratePartialMatch verifies that partially matched entries are migrated
// and unmatched entries are logged without error.
func TestMigratePartialMatch(t *testing.T) {
	dir := t.TempDir()

	// Old sessions: 3 sessions, 2 have mappings, 1 does not.
	oldSessions := map[string]interface{}{
		"sessions": []map[string]interface{}{
			{
				"session_id":  "sess-match-1",
				"saved_at":    "2026-03-01T10:00:00Z",
				"working_dir": "/proj/x",
				"title":       "Matched 1",
				"channel_id":  100,
			},
			{
				"session_id":  "sess-match-2",
				"saved_at":    "2026-03-02T10:00:00Z",
				"working_dir": "/proj/y",
				"title":       "Matched 2",
				"channel_id":  200,
			},
			{
				"session_id":  "sess-nomatch",
				"saved_at":    "2026-03-03T10:00:00Z",
				"working_dir": "/proj/z",
				"title":       "No match",
				"channel_id":  999,
			},
		},
	}
	sessPath := writeTempJSON(t, dir, "sessions.json", oldSessions)

	// Only 2 of the 3 channels have mappings.
	mappings := map[string]interface{}{
		"mappings": map[string]interface{}{
			"100": map[string]interface{}{
				"path": "/proj/x",
				"name": "project-x",
			},
			"200": map[string]interface{}{
				"path": "/proj/y",
				"name": "project-y",
			},
		},
	}
	mappingsPath := writeTempJSON(t, dir, "mappings.json", mappings)

	result, err := MigrateSessionHistory(sessPath, mappingsPath)
	if err != nil {
		t.Fatalf("MigrateSessionHistory: %v", err)
	}

	if result.Migrated != 2 {
		t.Errorf("Migrated: want 2, got %d", result.Migrated)
	}
	if result.Unmapped != 1 {
		t.Errorf("Unmapped: want 1, got %d", result.Unmapped)
	}
	if len(result.UnmappedEntries) != 1 {
		t.Errorf("UnmappedEntries: want 1, got %d", len(result.UnmappedEntries))
	}

	// Output should have 2 migrated sessions.
	h := readSessionHistoryFile(t, sessPath)
	if len(h.Sessions) != 2 {
		t.Fatalf("expected 2 sessions in output, got %d", len(h.Sessions))
	}
}

// TestMigrateMissingSessionsFile verifies that a missing sessions file results in
// zero counts and no error.
func TestMigrateMissingSessionsFile(t *testing.T) {
	dir := t.TempDir()

	mappings := map[string]interface{}{
		"mappings": map[string]interface{}{
			"12345": map[string]interface{}{
				"path": "/proj/a",
				"name": "proj-a",
			},
		},
	}
	mappingsPath := writeTempJSON(t, dir, "mappings.json", mappings)

	nonExistentSess := filepath.Join(dir, "no-such-sessions.json")

	result, err := MigrateSessionHistory(nonExistentSess, mappingsPath)
	if err != nil {
		t.Fatalf("MigrateSessionHistory: expected nil error for missing sessions file, got: %v", err)
	}
	if result.Migrated != 0 {
		t.Errorf("Migrated: want 0, got %d", result.Migrated)
	}
	if result.Unmapped != 0 {
		t.Errorf("Unmapped: want 0, got %d", result.Unmapped)
	}
}

// TestMigrateMissingMappingsFile verifies that a missing mappings file causes all
// sessions to be logged as unmappable (not an error).
func TestMigrateMissingMappingsFile(t *testing.T) {
	dir := t.TempDir()

	oldSessions := map[string]interface{}{
		"sessions": []map[string]interface{}{
			{
				"session_id":  "sess-orphan",
				"saved_at":    "2026-04-01T10:00:00Z",
				"working_dir": "/proj/orphan",
				"title":       "Orphan session",
				"channel_id":  555555,
			},
		},
	}
	sessPath := writeTempJSON(t, dir, "sessions.json", oldSessions)

	nonExistentMappings := filepath.Join(dir, "no-such-mappings.json")

	result, err := MigrateSessionHistory(sessPath, nonExistentMappings)
	if err != nil {
		t.Fatalf("MigrateSessionHistory: expected nil error for missing mappings file, got: %v", err)
	}
	if result.Migrated != 0 {
		t.Errorf("Migrated: want 0, got %d", result.Migrated)
	}
	if result.Unmapped != 1 {
		t.Errorf("Unmapped: want 1, got %d", result.Unmapped)
	}
	if len(result.UnmappedEntries) != 1 {
		t.Errorf("UnmappedEntries: want 1, got %d", len(result.UnmappedEntries))
	}
	// Output file should have empty sessions since all were unmapped.
	h := readSessionHistoryFile(t, sessPath)
	if len(h.Sessions) != 0 {
		t.Errorf("expected 0 sessions in output, got %d", len(h.Sessions))
	}
}
