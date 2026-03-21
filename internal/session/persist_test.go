package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// newTestPM creates a PersistenceManager pointing at a temp file.
// The returned cleanup function removes the temp directory.
func newTestPM(t *testing.T, maxPerProject int) (*PersistenceManager, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session-history.json")
	pm := NewPersistenceManager(path, maxPerProject)
	return pm, func() { os.RemoveAll(dir) }
}

// makeSession creates a SavedSession with the given parameters for test convenience.
func makeSession(id, workingDir, savedAt string, instanceID string) SavedSession {
	return SavedSession{
		SessionID:  id,
		WorkingDir: workingDir,
		SavedAt:    savedAt,
		Title:      "Test session " + id,
		InstanceID: instanceID,
	}
}

// TestPersistenceSaveAndLoad verifies a round-trip: save then load returns the same data.
func TestPersistenceSaveAndLoad(t *testing.T) {
	pm, cleanup := newTestPM(t, 5)
	defer cleanup()

	sess := makeSession("sess-001", "/proj/a", "2026-01-01T10:00:00Z", "inst-100")
	if err := pm.Save(sess); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	history, err := pm.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(history.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(history.Sessions))
	}
	got := history.Sessions[0]
	if got.SessionID != sess.SessionID {
		t.Errorf("SessionID: want %q, got %q", sess.SessionID, got.SessionID)
	}
	if got.WorkingDir != sess.WorkingDir {
		t.Errorf("WorkingDir: want %q, got %q", sess.WorkingDir, got.WorkingDir)
	}
	if got.InstanceID != sess.InstanceID {
		t.Errorf("InstanceID: want %q, got %q", sess.InstanceID, got.InstanceID)
	}
}

// TestPersistenceAppend verifies that saving two distinct sessions retains both.
func TestPersistenceAppend(t *testing.T) {
	pm, cleanup := newTestPM(t, 5)
	defer cleanup()

	if err := pm.Save(makeSession("s1", "/proj/a", "2026-01-01T10:00:00Z", "inst-1")); err != nil {
		t.Fatalf("Save s1: %v", err)
	}
	if err := pm.Save(makeSession("s2", "/proj/a", "2026-01-01T11:00:00Z", "inst-1")); err != nil {
		t.Fatalf("Save s2: %v", err)
	}

	history, err := pm.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(history.Sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(history.Sessions))
	}
}

// TestPersistenceMaxPerProject verifies that only the 5 most recent sessions
// are kept per working dir when more than 5 are saved.
func TestPersistenceMaxPerProject(t *testing.T) {
	pm, cleanup := newTestPM(t, 5)
	defer cleanup()

	// Save 7 sessions with incrementing timestamps.
	for i := 1; i <= 7; i++ {
		ts := time.Date(2026, 1, i, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)
		id := "sess-" + ts
		if err := pm.Save(makeSession(id, "/proj/trim", ts, "inst-200")); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}

	history, err := pm.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Only 5 should remain (most recent).
	count := 0
	for _, s := range history.Sessions {
		if s.WorkingDir == "/proj/trim" {
			count++
		}
	}
	if count != 5 {
		t.Errorf("expected 5 sessions for /proj/trim, got %d", count)
	}

	// Verify the oldest two (days 1 and 2) were dropped.
	for _, s := range history.Sessions {
		if s.WorkingDir == "/proj/trim" {
			if s.SavedAt < "2026-01-03" {
				t.Errorf("expected day 1 and 2 to be trimmed, but found %s", s.SavedAt)
			}
		}
	}
}

// TestPersistenceMultipleProjects verifies that trim limits are applied independently
// per working directory.
func TestPersistenceMultipleProjects(t *testing.T) {
	pm, cleanup := newTestPM(t, 3)
	defer cleanup()

	// Save 5 for project A and 5 for project B.
	for i := 1; i <= 5; i++ {
		ts := time.Date(2026, 2, i, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
		pm.Save(makeSession("a"+ts, "/proj/alpha", ts, "inst-300")) //nolint:errcheck
		pm.Save(makeSession("b"+ts, "/proj/beta", ts, "inst-301"))  //nolint:errcheck
	}

	history, err := pm.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	countA, countB := 0, 0
	for _, s := range history.Sessions {
		switch s.WorkingDir {
		case "/proj/alpha":
			countA++
		case "/proj/beta":
			countB++
		}
	}
	if countA != 3 {
		t.Errorf("expected 3 for alpha, got %d", countA)
	}
	if countB != 3 {
		t.Errorf("expected 3 for beta, got %d", countB)
	}
}

// TestPersistenceAtomicWrite verifies that the file is always valid JSON after a save.
func TestPersistenceAtomicWrite(t *testing.T) {
	pm, cleanup := newTestPM(t, 5)
	defer cleanup()

	// Save a session.
	if err := pm.Save(makeSession("atomic-1", "/proj/atomic", "2026-03-01T00:00:00Z", "inst-400")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read raw bytes and unmarshal — must be valid JSON.
	data, err := os.ReadFile(pm.filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var h SessionHistory
	if err := json.Unmarshal(data, &h); err != nil {
		t.Errorf("file is not valid JSON after save: %v\ncontent:\n%s", err, data)
	}
}

// TestPersistenceConcurrentSave verifies that 10 goroutines each saving 5 sessions
// does not panic and leaves a valid JSON file.
func TestPersistenceConcurrentSave(t *testing.T) {
	pm, cleanup := newTestPM(t, 20) // high limit so all sessions can coexist
	defer cleanup()

	const goroutines = 10
	const savesPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < savesPerGoroutine; i++ {
				ts := time.Now().UTC().Format(time.RFC3339Nano)
				id := "concurrent-" + string(rune('A'+g)) + "-" + ts
				instanceID := "inst-" + string(rune('A'+g))
				sess := makeSession(id, "/proj/concurrent", ts, instanceID)
				if err := pm.Save(sess); err != nil {
					t.Errorf("goroutine %d save %d: %v", g, i, err)
				}
			}
		}()
	}
	wg.Wait()

	// File must be valid JSON.
	data, err := os.ReadFile(pm.filePath)
	if err != nil {
		t.Fatalf("ReadFile after concurrent saves: %v", err)
	}
	var h SessionHistory
	if err := json.Unmarshal(data, &h); err != nil {
		t.Errorf("file not valid JSON after concurrent saves: %v", err)
	}
}

// TestPersistenceLoadMissingFile verifies that Load returns an empty history (no error)
// when the file does not exist.
func TestPersistenceLoadMissingFile(t *testing.T) {
	pm := NewPersistenceManager("/nonexistent/path/to/sessions.json", 5)

	history, err := pm.Load()
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
	if history == nil {
		t.Fatal("expected non-nil SessionHistory")
	}
	if len(history.Sessions) != 0 {
		t.Errorf("expected empty sessions, got %d", len(history.Sessions))
	}
}

// TestLoadForInstance verifies that LoadForInstance filters to the correct instance.
func TestLoadForInstance(t *testing.T) {
	pm, cleanup := newTestPM(t, 5)
	defer cleanup()

	pm.Save(makeSession("inst1-s1", "/proj/x", "2026-04-01T00:00:00Z", "inst-1")) //nolint:errcheck
	pm.Save(makeSession("inst1-s2", "/proj/x", "2026-04-02T00:00:00Z", "inst-1")) //nolint:errcheck
	pm.Save(makeSession("inst2-s1", "/proj/x", "2026-04-01T00:00:00Z", "inst-2")) //nolint:errcheck

	inst1, err := pm.LoadForInstance("inst-1")
	if err != nil {
		t.Fatalf("LoadForInstance(inst-1): %v", err)
	}
	if len(inst1) != 2 {
		t.Errorf("expected 2 sessions for inst-1, got %d", len(inst1))
	}

	inst2, err := pm.LoadForInstance("inst-2")
	if err != nil {
		t.Fatalf("LoadForInstance(inst-2): %v", err)
	}
	if len(inst2) != 1 {
		t.Errorf("expected 1 session for inst-2, got %d", len(inst2))
	}
}

// TestGetLatestForInstance verifies that GetLatestForInstance returns the session
// with the latest SavedAt timestamp.
func TestGetLatestForInstance(t *testing.T) {
	pm, cleanup := newTestPM(t, 5)
	defer cleanup()

	pm.Save(makeSession("old", "/proj/y", "2026-05-01T00:00:00Z", "inst-10"))    //nolint:errcheck
	pm.Save(makeSession("middle", "/proj/y", "2026-05-02T00:00:00Z", "inst-10")) //nolint:errcheck
	pm.Save(makeSession("latest", "/proj/y", "2026-05-03T00:00:00Z", "inst-10")) //nolint:errcheck

	got, err := pm.GetLatestForInstance("inst-10")
	if err != nil {
		t.Fatalf("GetLatestForInstance: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil session")
	}
	if got.SessionID != "latest" {
		t.Errorf("expected SessionID=latest, got %q", got.SessionID)
	}
}

// TestGetLatestForInstanceEmpty verifies nil is returned when no sessions exist for instance.
func TestGetLatestForInstanceEmpty(t *testing.T) {
	pm, cleanup := newTestPM(t, 5)
	defer cleanup()

	got, err := pm.GetLatestForInstance("unknown-inst")
	if err != nil {
		t.Fatalf("GetLatestForInstance: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unknown instance, got %+v", got)
	}
}
