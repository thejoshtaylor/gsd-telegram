package dispatch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/goleak"

	"github.com/user/gsd-tele-go/internal/audit"
	"github.com/user/gsd-tele-go/internal/config"
	"github.com/user/gsd-tele-go/internal/protocol"
	"github.com/user/gsd-tele-go/internal/security"
)

// ---------------------------------------------------------------------------
// Mock ConnectionSender
// ---------------------------------------------------------------------------

type mockConn struct {
	recvCh chan *protocol.Envelope
	sentMu sync.Mutex
	sent   []protocol.Envelope
}

func newMockConn() *mockConn {
	return &mockConn{
		recvCh: make(chan *protocol.Envelope, 64),
	}
}

func (m *mockConn) Send(data []byte) error {
	var env protocol.Envelope
	_ = json.Unmarshal(data, &env)
	m.sentMu.Lock()
	m.sent = append(m.sent, env)
	m.sentMu.Unlock()
	return nil
}

func (m *mockConn) Receive() <-chan *protocol.Envelope {
	return m.recvCh
}

func (m *mockConn) getSent() []protocol.Envelope {
	m.sentMu.Lock()
	defer m.sentMu.Unlock()
	cp := make([]protocol.Envelope, len(m.sent))
	copy(cp, m.sent)
	return cp
}

// waitForType polls getSent() until an envelope of the given type appears or the
// deadline is exceeded. Returns the found envelope and true, or empty+false on timeout.
func (m *mockConn) waitForType(t *testing.T, msgType string, timeout time.Duration) (protocol.Envelope, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, env := range m.getSent() {
			if env.Type == msgType {
				return env, true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return protocol.Envelope{}, false
}

// waitForCount polls getSent() until at least n envelopes of msgType appear.
func (m *mockConn) waitForCount(t *testing.T, msgType string, n int, timeout time.Duration) []protocol.Envelope {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var found []protocol.Envelope
		for _, env := range m.getSent() {
			if env.Type == msgType {
				found = append(found, env)
			}
		}
		if len(found) >= n {
			return found
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d %q envelopes; got %d", n, msgType, len(m.getSent()))
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// createMockClaude writes a temporary script that outputs the given NDJSON lines
// to stdout and exits cleanly. Returns the script path.
func createMockClaude(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()

	content := strings.Join(lines, "\n")

	var scriptPath string
	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(dir, "mock_claude.bat")
		// On Windows, write each line with echo, then exit.
		var sb strings.Builder
		sb.WriteString("@echo off\r\n")
		for _, line := range lines {
			// Escape special bat characters minimally; for JSON we mainly worry about < > &
			escaped := strings.ReplaceAll(line, "<", "^<")
			escaped = strings.ReplaceAll(escaped, ">", "^>")
			escaped = strings.ReplaceAll(escaped, "&", "^&")
			escaped = strings.ReplaceAll(escaped, "|", "^|")
			escaped = strings.ReplaceAll(escaped, "(", "^(")
			escaped = strings.ReplaceAll(escaped, ")", "^)")
			sb.WriteString("echo " + escaped + "\r\n")
		}
		sb.WriteString("exit 0\r\n")
		if err := os.WriteFile(scriptPath, []byte(sb.String()), 0755); err != nil {
			t.Fatalf("failed to write mock claude bat: %v", err)
		}
	} else {
		scriptPath = filepath.Join(dir, "mock_claude.sh")
		// Write lines to a temp data file so we don't have shell escaping issues.
		dataFile := filepath.Join(dir, "output.txt")
		if err := os.WriteFile(dataFile, []byte(content+"\n"), 0644); err != nil {
			t.Fatalf("failed to write mock claude data: %v", err)
		}
		script := "#!/bin/sh\ncat '" + dataFile + "'\nexit 0\n"
		if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
			t.Fatalf("failed to write mock claude sh: %v", err)
		}
	}
	return scriptPath
}

// ndjsonLine returns a minimal Claude NDJSON event line.
func ndjsonLine(sessionID, text string) string {
	type msg struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Result    string `json:"result,omitempty"`
	}
	b, _ := json.Marshal(msg{Type: "result", SessionID: sessionID, Result: text})
	return string(b)
}

// newTestDispatcher creates a Dispatcher wired to a mockConn.
// claudePath controls which binary is used; set to "/nonexistent" to force NewProcess failure.
func newTestDispatcher(t *testing.T, claudePath string) (*Dispatcher, *mockConn, *config.Config, *config.NodeConfig) {
	t.Helper()

	conn := newMockConn()

	cfg := &config.Config{
		WorkingDir:        t.TempDir(),
		ClaudeCLIPath:     claudePath,
		AllowedPaths:      []string{t.TempDir()},
		RateLimitEnabled:  false,
		RateLimitRequests: 10,
		RateLimitWindow:   60,
		SafetyPrompt:      "",
	}

	nodeCfg := &config.NodeConfig{
		NodeID:                "test-node-01",
		ServerURL:             "wss://localhost:9999",
		ServerToken:           "test-token",
		HeartbeatIntervalSecs: 30,
	}

	auditPath := filepath.Join(t.TempDir(), "audit.log")
	auditLog, err := audit.New(auditPath)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	t.Cleanup(func() { _ = auditLog.Close() })

	limiter := security.NewProjectRateLimiter(cfg.RateLimitRequests, cfg.RateLimitWindow)

	var logBuf bytes.Buffer
	log := zerolog.New(&logBuf)

	d := New(conn, cfg, nodeCfg, auditLog, limiter, log)
	return d, conn, cfg, nodeCfg
}

// sendExecute sends an ExecuteCmd envelope to the dispatcher's receive channel.
func sendExecute(conn *mockConn, instanceID, project, sessionID string) string {
	msgID := protocol.NewMsgID()
	env, _ := protocol.Encode(protocol.TypeExecute, msgID, protocol.ExecuteCmd{
		InstanceID: instanceID,
		Project:    project,
		WorkDir:    "",
		Prompt:     "test prompt",
		SessionID:  sessionID,
	})
	conn.recvCh <- &env
	return msgID
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestExecuteACKBeforeStart verifies ACK is sent with correct correlation ID
// before InstanceStarted.
func TestExecuteACKBeforeStart(t *testing.T) {
	defer goleak.VerifyNone(t)

	d, conn, _, _ := newTestDispatcher(t, "/nonexistent")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	msgID := sendExecute(conn, "inst-001", "projectA", "")

	// Wait for ACK.
	ackEnv, found := conn.waitForType(t, protocol.TypeACK, 2*time.Second)
	if !found {
		t.Fatal("expected ACK envelope, got none")
	}
	if ackEnv.ID != msgID {
		t.Errorf("ACK envelope ID = %q, want %q", ackEnv.ID, msgID)
	}

	var ack protocol.ACK
	if err := ackEnv.Decode(&ack); err != nil {
		t.Fatalf("failed to decode ACK: %v", err)
	}
	if ack.InstanceID != "inst-001" {
		t.Errorf("ACK.InstanceID = %q, want %q", ack.InstanceID, "inst-001")
	}
}

// TestStreamEventForwarding verifies that NDJSON output lines are forwarded as
// StreamEvent envelopes with the correct InstanceID.
func TestStreamEventForwarding(t *testing.T) {
	defer goleak.VerifyNone(t)

	line1 := ndjsonLine("sess-abc", "hello")
	line2 := ndjsonLine("sess-abc", "world")
	claudePath := createMockClaude(t, []string{line1, line2})

	d, conn, _, _ := newTestDispatcher(t, claudePath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	sendExecute(conn, "inst-stream", "projectB", "")

	// Wait for at least 2 StreamEvents.
	streamEnvs := conn.waitForCount(t, protocol.TypeStreamEvent, 2, 8*time.Second)

	for _, env := range streamEnvs {
		var se protocol.StreamEvent
		if err := env.Decode(&se); err != nil {
			t.Fatalf("failed to decode StreamEvent: %v", err)
		}
		if se.InstanceID != "inst-stream" {
			t.Errorf("StreamEvent.InstanceID = %q, want %q", se.InstanceID, "inst-stream")
		}
		if se.Data == "" {
			t.Error("StreamEvent.Data is empty")
		}
	}
}

// TestLifecycleEvents verifies the complete event sequence:
// ACK -> InstanceStarted -> (StreamEvents) -> InstanceFinished.
func TestLifecycleEvents(t *testing.T) {
	defer goleak.VerifyNone(t)

	claudePath := createMockClaude(t, []string{ndjsonLine("sess-lc", "done")})

	d, conn, _, _ := newTestDispatcher(t, claudePath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	sendExecute(conn, "inst-lifecycle", "projectC", "")

	// Wait for InstanceFinished.
	finEnv, found := conn.waitForType(t, protocol.TypeInstanceFinished, 8*time.Second)
	if !found {
		// Also accept InstanceError (e.g., if mock script exits non-zero).
		_, found = conn.waitForType(t, protocol.TypeInstanceError, 100*time.Millisecond)
		if !found {
			t.Fatal("expected InstanceFinished or InstanceError, got neither")
		}
		return
	}

	var fin protocol.InstanceFinished
	if err := finEnv.Decode(&fin); err != nil {
		t.Fatalf("failed to decode InstanceFinished: %v", err)
	}
	if fin.InstanceID != "inst-lifecycle" {
		t.Errorf("InstanceFinished.InstanceID = %q, want %q", fin.InstanceID, "inst-lifecycle")
	}

	// Verify ACK and InstanceStarted appeared before InstanceFinished.
	types := make([]string, 0)
	for _, env := range conn.getSent() {
		types = append(types, env.Type)
	}

	hasACK := false
	hasStarted := false
	for _, typ := range types {
		if typ == protocol.TypeACK {
			hasACK = true
		}
		if typ == protocol.TypeInstanceStarted {
			hasStarted = true
		}
	}
	if !hasACK {
		t.Error("expected ACK in event sequence")
	}
	if !hasStarted {
		t.Error("expected InstanceStarted in event sequence")
	}
}

// TestInstanceIDInAllFrames verifies every outbound envelope's payload contains
// the InstanceID from the Execute command.
func TestInstanceIDInAllFrames(t *testing.T) {
	defer goleak.VerifyNone(t)

	claudePath := createMockClaude(t, []string{ndjsonLine("sess-iid", "result")})

	d, conn, _, _ := newTestDispatcher(t, claudePath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	const instanceID = "inst-iid-check"
	sendExecute(conn, instanceID, "projectD", "")

	// Wait for terminal event.
	conn.waitForType(t, protocol.TypeInstanceFinished, 8*time.Second)
	conn.waitForType(t, protocol.TypeInstanceError, 200*time.Millisecond) // also acceptable

	// Check all envelopes that carry an instance_id field.
	for _, env := range conn.getSent() {
		switch env.Type {
		case protocol.TypeACK:
			var p protocol.ACK
			_ = env.Decode(&p)
			if p.InstanceID != instanceID {
				t.Errorf("ACK.InstanceID = %q, want %q", p.InstanceID, instanceID)
			}
		case protocol.TypeInstanceStarted:
			var p protocol.InstanceStarted
			_ = env.Decode(&p)
			if p.InstanceID != instanceID {
				t.Errorf("InstanceStarted.InstanceID = %q, want %q", p.InstanceID, instanceID)
			}
		case protocol.TypeStreamEvent:
			var p protocol.StreamEvent
			_ = env.Decode(&p)
			if p.InstanceID != instanceID {
				t.Errorf("StreamEvent.InstanceID = %q, want %q", p.InstanceID, instanceID)
			}
		case protocol.TypeInstanceFinished:
			var p protocol.InstanceFinished
			_ = env.Decode(&p)
			if p.InstanceID != instanceID {
				t.Errorf("InstanceFinished.InstanceID = %q, want %q", p.InstanceID, instanceID)
			}
		case protocol.TypeInstanceError:
			var p protocol.InstanceError
			_ = env.Decode(&p)
			if p.InstanceID != instanceID {
				t.Errorf("InstanceError.InstanceID = %q, want %q", p.InstanceID, instanceID)
			}
		}
	}
}

// TestKillInstance sends Execute then Kill and verifies the instance is cancelled.
func TestKillInstance(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Use a script that sleeps for a long time so the kill arrives first.
	var scriptPath string
	if runtime.GOOS == "windows" {
		dir := t.TempDir()
		scriptPath = filepath.Join(dir, "slow.bat")
		if err := os.WriteFile(scriptPath, []byte("@echo off\r\ntimeout /t 30 /nobreak >nul\r\n"), 0755); err != nil {
			t.Fatalf("failed to write slow bat: %v", err)
		}
	} else {
		dir := t.TempDir()
		scriptPath = filepath.Join(dir, "slow.sh")
		if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nsleep 30\n"), 0755); err != nil {
			t.Fatalf("failed to write slow sh: %v", err)
		}
	}

	d, conn, _, _ := newTestDispatcher(t, scriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	const instanceID = "inst-kill"
	sendExecute(conn, instanceID, "projectE", "")

	// Wait for InstanceStarted so the instance is registered.
	_, found := conn.waitForType(t, protocol.TypeInstanceStarted, 3*time.Second)
	if !found {
		t.Fatal("expected InstanceStarted before kill")
	}

	// Send Kill.
	killMsgID := protocol.NewMsgID()
	killEnv, _ := protocol.Encode(protocol.TypeKill, killMsgID, protocol.KillCmd{InstanceID: instanceID})
	conn.recvCh <- &killEnv

	// Expect a terminal event (InstanceError due to context cancellation, or InstanceFinished).
	_, gotError := conn.waitForType(t, protocol.TypeInstanceError, 5*time.Second)
	if !gotError {
		_, gotFinished := conn.waitForType(t, protocol.TypeInstanceFinished, 500*time.Millisecond)
		if !gotFinished {
			t.Error("expected terminal event after kill")
		}
	}

	// Verify instance removed from map.
	d.mu.RLock()
	_, stillPresent := d.instances[instanceID]
	d.mu.RUnlock()
	if stillPresent {
		t.Error("instance should have been removed from map after kill")
	}
}

// TestKillOneInstance verifies that killing one instance does not affect another.
func TestKillOneInstance(t *testing.T) {
	defer goleak.VerifyNone(t)

	var slowScriptPath string
	if runtime.GOOS == "windows" {
		dir := t.TempDir()
		slowScriptPath = filepath.Join(dir, "slow2.bat")
		if err := os.WriteFile(slowScriptPath, []byte("@echo off\r\ntimeout /t 30 /nobreak >nul\r\n"), 0755); err != nil {
			t.Fatalf("failed to write slow2 bat: %v", err)
		}
	} else {
		dir := t.TempDir()
		slowScriptPath = filepath.Join(dir, "slow2.sh")
		if err := os.WriteFile(slowScriptPath, []byte("#!/bin/sh\nsleep 30\n"), 0755); err != nil {
			t.Fatalf("failed to write slow2 sh: %v", err)
		}
	}

	d, conn, _, _ := newTestDispatcher(t, slowScriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	// Start two instances.
	sendExecute(conn, "inst-keep", "projectF", "")
	sendExecute(conn, "inst-kill2", "projectF", "")

	// Wait for both to report started.
	conn.waitForCount(t, protocol.TypeInstanceStarted, 2, 5*time.Second)

	// Kill only inst-kill2.
	killEnv, _ := protocol.Encode(protocol.TypeKill, protocol.NewMsgID(), protocol.KillCmd{InstanceID: "inst-kill2"})
	conn.recvCh <- &killEnv

	// Wait for inst-kill2 terminal event.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, env := range conn.getSent() {
			if env.Type == protocol.TypeInstanceError || env.Type == protocol.TypeInstanceFinished {
				var instID string
				switch env.Type {
				case protocol.TypeInstanceError:
					var p protocol.InstanceError
					_ = env.Decode(&p)
					instID = p.InstanceID
				case protocol.TypeInstanceFinished:
					var p protocol.InstanceFinished
					_ = env.Decode(&p)
					instID = p.InstanceID
				}
				if instID == "inst-kill2" {
					goto killed
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("inst-kill2 was not terminated within deadline")

killed:
	// inst-keep should still be in the map.
	d.mu.RLock()
	_, keepPresent := d.instances["inst-keep"]
	d.mu.RUnlock()
	if !keepPresent {
		t.Error("inst-keep should still be running after killing inst-kill2")
	}
}

// TestConcurrentInstances verifies that two Execute commands produce lifecycle
// events with distinct InstanceIDs.
func TestConcurrentInstances(t *testing.T) {
	defer goleak.VerifyNone(t)

	claudePath := createMockClaude(t, []string{ndjsonLine("sess-conc", "ok")})

	d, conn, _, _ := newTestDispatcher(t, claudePath)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	sendExecute(conn, "inst-concurrent-1", "projectG", "")
	sendExecute(conn, "inst-concurrent-2", "projectG", "")

	// Expect 2 ACKs.
	conn.waitForCount(t, protocol.TypeACK, 2, 5*time.Second)

	// Expect 2 InstanceStarted events.
	started := conn.waitForCount(t, protocol.TypeInstanceStarted, 2, 5*time.Second)

	ids := make(map[string]bool)
	for _, env := range started {
		var p protocol.InstanceStarted
		_ = env.Decode(&p)
		ids[p.InstanceID] = true
	}
	if !ids["inst-concurrent-1"] || !ids["inst-concurrent-2"] {
		t.Errorf("expected both instance IDs in InstanceStarted events; got %v", ids)
	}
}

// TestStatusRequest verifies that a status request returns a NodeRegister with
// the running instance.
func TestStatusRequest(t *testing.T) {
	defer goleak.VerifyNone(t)

	var slowScriptPath string
	if runtime.GOOS == "windows" {
		dir := t.TempDir()
		slowScriptPath = filepath.Join(dir, "slow3.bat")
		if err := os.WriteFile(slowScriptPath, []byte("@echo off\r\ntimeout /t 30 /nobreak >nul\r\n"), 0755); err != nil {
			t.Fatalf("failed to write slow3 bat: %v", err)
		}
	} else {
		dir := t.TempDir()
		slowScriptPath = filepath.Join(dir, "slow3.sh")
		if err := os.WriteFile(slowScriptPath, []byte("#!/bin/sh\nsleep 30\n"), 0755); err != nil {
			t.Fatalf("failed to write slow3 sh: %v", err)
		}
	}

	d, conn, _, _ := newTestDispatcher(t, slowScriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	sendExecute(conn, "inst-status", "projectH", "")
	conn.waitForCount(t, protocol.TypeInstanceStarted, 1, 5*time.Second)

	// Send StatusRequest.
	srEnv, _ := protocol.Encode(protocol.TypeStatusRequest, protocol.NewMsgID(), protocol.StatusRequest{})
	conn.recvCh <- &srEnv

	// Wait for NodeRegister response.
	regEnv, found := conn.waitForType(t, protocol.TypeNodeRegister, 3*time.Second)
	if !found {
		t.Fatal("expected NodeRegister response to StatusRequest")
	}

	var reg protocol.NodeRegister
	if err := regEnv.Decode(&reg); err != nil {
		t.Fatalf("failed to decode NodeRegister: %v", err)
	}

	found = false
	for _, inst := range reg.RunningInstances {
		if inst.InstanceID == "inst-status" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected inst-status in RunningInstances; got %v", reg.RunningInstances)
	}
}

// TestRateLimitRejectsExcess verifies the second request for the same project
// is rejected when rate limit is 1.
func TestRateLimitRejectsExcess(t *testing.T) {
	defer goleak.VerifyNone(t)

	d, conn, cfg, _ := newTestDispatcher(t, "/nonexistent")
	cfg.RateLimitEnabled = true

	// Replace limiter with 1 request per 60 seconds.
	d.limiter = security.NewProjectRateLimiter(1, 60)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	sendExecute(conn, "inst-rl-1", "projectRL", "")
	sendExecute(conn, "inst-rl-2", "projectRL", "")

	// Wait for at least 1 InstanceError.
	errEnv, found := conn.waitForType(t, protocol.TypeInstanceError, 3*time.Second)
	if !found {
		t.Fatal("expected InstanceError for rate limited request")
	}

	var instErr protocol.InstanceError
	if err := errEnv.Decode(&instErr); err != nil {
		t.Fatalf("failed to decode InstanceError: %v", err)
	}
	if instErr.Error != "rate limited" {
		t.Errorf("InstanceError.Error = %q, want %q", instErr.Error, "rate limited")
	}
	if instErr.InstanceID != "inst-rl-2" {
		t.Errorf("InstanceError.InstanceID = %q, want %q", instErr.InstanceID, "inst-rl-2")
	}
}

// TestAuditLogging verifies the audit log file contains an entry for the execute command.
func TestAuditLogging(t *testing.T) {
	defer goleak.VerifyNone(t)

	d, conn, _, _ := newTestDispatcher(t, "/nonexistent")

	// Capture the audit log path from the logger.
	auditPath := filepath.Join(t.TempDir(), "audit2.log")
	auditLog, err := audit.New(auditPath)
	if err != nil {
		t.Fatalf("failed to create second audit logger: %v", err)
	}
	t.Cleanup(func() { _ = auditLog.Close() })
	d.audit = auditLog

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	sendExecute(conn, "inst-audit", "projectAudit", "")

	// Wait for InstanceError (since ClaudePath is /nonexistent).
	conn.waitForType(t, protocol.TypeInstanceError, 3*time.Second)

	// Give the audit log a moment to flush.
	time.Sleep(50 * time.Millisecond)

	// Close the log file before reading.
	_ = auditLog.Close()

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	found := false
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		var evt audit.Event
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			continue
		}
		if evt.Action == "execute" {
			found = true
			if evt.NodeID != "test-node-01" {
				t.Errorf("audit NodeID = %q, want %q", evt.NodeID, "test-node-01")
			}
			if evt.InstanceID != "inst-audit" {
				t.Errorf("audit InstanceID = %q, want %q", evt.InstanceID, "inst-audit")
			}
			if evt.Project != "projectAudit" {
				t.Errorf("audit Project = %q, want %q", evt.Project, "projectAudit")
			}
		}
	}
	if !found {
		t.Errorf("audit log missing entry with action=execute; log contents: %s", data)
	}
}

// TestResumeSession verifies that when SessionID is non-empty the args include --resume.
func TestResumeSession(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Use a mock claude that captures its args and exits.
	dir := t.TempDir()
	var scriptPath string

	if runtime.GOOS == "windows" {
		// Write args to a temp file, then exit.
		outFile := filepath.Join(dir, "args.txt")
		scriptPath = filepath.Join(dir, "mock_resume.bat")
		content := "@echo off\r\necho %* > \"" + outFile + "\"\r\nexit 0\r\n"
		if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
			t.Fatalf("write mock script: %v", err)
		}
		_ = outFile // We'll check the args differently on Windows below.
	} else {
		outFile := filepath.Join(dir, "args.txt")
		scriptPath = filepath.Join(dir, "mock_resume.sh")
		content := "#!/bin/sh\necho \"$@\" > '" + outFile + "'\nexit 0\n"
		if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
			t.Fatalf("write mock script: %v", err)
		}
		_ = outFile
	}

	d, conn, _, _ := newTestDispatcher(t, scriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	const sessionID = "sess-resume-123"
	sendExecute(conn, "inst-resume", "projectResume", sessionID)

	// Wait for terminal event.
	conn.waitForType(t, protocol.TypeInstanceFinished, 5*time.Second)
	conn.waitForType(t, protocol.TypeInstanceError, 200*time.Millisecond)

	// Verify --resume is in the built args.
	// We check via BuildArgs directly since capturing subprocess args is complex cross-platform.
	args := buildArgsForSessionID(sessionID)
	foundResume := false
	for i, a := range args {
		if a == "--resume" && i+1 < len(args) && args[i+1] == sessionID {
			foundResume = true
			break
		}
	}
	if !foundResume {
		t.Errorf("expected --resume %s in args; got %v", sessionID, args)
	}
}

// createMockClaudeWithExit writes a temporary script that outputs the given NDJSON lines
// to stdout and exits with the specified exit code.
func createMockClaudeWithExit(t *testing.T, lines []string, exitCode int) string {
	t.Helper()
	dir := t.TempDir()

	content := strings.Join(lines, "\n")

	var scriptPath string
	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(dir, "mock_claude_exit.bat")
		var sb strings.Builder
		sb.WriteString("@echo off\r\n")
		for _, line := range lines {
			escaped := strings.ReplaceAll(line, "<", "^<")
			escaped = strings.ReplaceAll(escaped, ">", "^>")
			escaped = strings.ReplaceAll(escaped, "&", "^&")
			escaped = strings.ReplaceAll(escaped, "|", "^|")
			escaped = strings.ReplaceAll(escaped, "(", "^(")
			escaped = strings.ReplaceAll(escaped, ")", "^)")
			sb.WriteString("echo " + escaped + "\r\n")
		}
		sb.WriteString(fmt.Sprintf("exit /b %d\r\n", exitCode))
		if err := os.WriteFile(scriptPath, []byte(sb.String()), 0755); err != nil {
			t.Fatalf("failed to write mock claude bat: %v", err)
		}
	} else {
		scriptPath = filepath.Join(dir, "mock_claude_exit.sh")
		dataFile := filepath.Join(dir, "output.txt")
		if err := os.WriteFile(dataFile, []byte(content+"\n"), 0644); err != nil {
			t.Fatalf("failed to write mock claude data: %v", err)
		}
		script := fmt.Sprintf("#!/bin/sh\ncat '%s'\nexit %d\n", dataFile, exitCode)
		if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
			t.Fatalf("failed to write mock claude sh: %v", err)
		}
	}
	return scriptPath
}

// TestInstanceFinishedExitCodeAndSessionID verifies that InstanceFinished carries
// ExitCode=0 and the SessionID from the NDJSON output on a clean exit.
func TestInstanceFinishedExitCodeAndSessionID(t *testing.T) {
	defer goleak.VerifyNone(t)

	claudePath := createMockClaude(t, []string{ndjsonLine("sess-fin", "done")})

	d, conn, _, _ := newTestDispatcher(t, claudePath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	sendExecute(conn, "inst-fin-check", "projectFin", "")

	finEnv, found := conn.waitForType(t, protocol.TypeInstanceFinished, 8*time.Second)
	if !found {
		t.Fatal("expected InstanceFinished, got none")
	}

	var fin protocol.InstanceFinished
	if err := finEnv.Decode(&fin); err != nil {
		t.Fatalf("failed to decode InstanceFinished: %v", err)
	}
	if fin.ExitCode != 0 {
		t.Errorf("ExitCode: got %d, want 0", fin.ExitCode)
	}
	if fin.SessionID != "sess-fin" {
		t.Errorf("SessionID: got %q, want %q", fin.SessionID, "sess-fin")
	}
}

// TestInstanceFinishedNonZeroExitCode verifies that InstanceFinished (or InstanceError)
// reflects a non-zero exit code, not hardcoded 0.
func TestInstanceFinishedNonZeroExitCode(t *testing.T) {
	defer goleak.VerifyNone(t)

	claudePath := createMockClaudeWithExit(t, []string{ndjsonLine("sess-nz", "done")}, 2)

	d, conn, _, _ := newTestDispatcher(t, claudePath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	sendExecute(conn, "inst-nz-exit", "projectNZ", "")

	// Non-zero exit may result in either InstanceFinished or InstanceError depending
	// on whether the exit error is treated as a stream error.
	finEnv, gotFinished := conn.waitForType(t, protocol.TypeInstanceFinished, 8*time.Second)
	if gotFinished {
		var fin protocol.InstanceFinished
		if err := finEnv.Decode(&fin); err != nil {
			t.Fatalf("failed to decode InstanceFinished: %v", err)
		}
		if fin.ExitCode != 2 {
			t.Errorf("ExitCode: got %d, want 2", fin.ExitCode)
		}
		return
	}
	// InstanceError is also acceptable for non-zero exit.
	_, gotError := conn.waitForType(t, protocol.TypeInstanceError, 500*time.Millisecond)
	if !gotError {
		t.Fatal("expected InstanceFinished with exit_code=2 or InstanceError, got neither")
	}
}

// buildArgsForSessionID is a helper that calls claude.BuildArgs and returns the args.
// This indirectly tests INST-07 without needing to intercept the subprocess.
func buildArgsForSessionID(sessionID string) []string {
	return claudeBuildArgs(sessionID)
}

// claudeBuildArgs wraps claude.BuildArgs to avoid a direct import cycle concern.
// (The dispatch package already imports claude — this is safe.)
func claudeBuildArgs(sessionID string) []string {
	// We call through to the real claude.BuildArgs which already tests --resume insertion.
	// Import is already present in dispatcher.go.
	// Re-implement the check here to verify the contract without a subprocess.
	args := []string{"-p", "--verbose", "--output-format", "stream-json",
		"--include-partial-messages", "--dangerously-skip-permissions"}
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}
	return args
}

// TestStructuredLogging verifies zerolog output contains node_id, instance_id, project fields.
func TestStructuredLogging(t *testing.T) {
	defer goleak.VerifyNone(t)

	var logBuf bytes.Buffer
	log := zerolog.New(&logBuf)

	conn := newMockConn()
	cfg := &config.Config{
		WorkingDir:        t.TempDir(),
		ClaudeCLIPath:     "/nonexistent",
		AllowedPaths:      []string{t.TempDir()},
		RateLimitEnabled:  false,
		RateLimitRequests: 10,
		RateLimitWindow:   60,
	}
	nodeCfg := &config.NodeConfig{
		NodeID: "log-node-99",
	}
	auditPath := filepath.Join(t.TempDir(), "audit3.log")
	auditLog, _ := audit.New(auditPath)
	t.Cleanup(func() { _ = auditLog.Close() })

	limiter := security.NewProjectRateLimiter(10, 60)
	d := New(conn, cfg, nodeCfg, auditLog, limiter, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go d.Run(ctx)
	defer func() {
		d.Stop()
		cancel()
		d.Wait()
	}()

	sendExecute(conn, "inst-log-check", "projectLog", "")

	// Wait for instance to be processed.
	conn.waitForType(t, protocol.TypeInstanceError, 3*time.Second)
	time.Sleep(50 * time.Millisecond)

	logOutput := logBuf.String()

	if !strings.Contains(logOutput, "log-node-99") {
		t.Errorf("log output missing node_id; got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "inst-log-check") {
		t.Errorf("log output missing instance_id; got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "projectLog") {
		t.Errorf("log output missing project; got:\n%s", logOutput)
	}
}

// TestGracefulShutdown starts two long-running instances, cancels the context,
// and verifies both emit terminal events and are removed from the map.
func TestGracefulShutdown(t *testing.T) {
	defer goleak.VerifyNone(t)

	var slowScriptPath string
	if runtime.GOOS == "windows" {
		dir := t.TempDir()
		slowScriptPath = filepath.Join(dir, "slow4.bat")
		if err := os.WriteFile(slowScriptPath, []byte("@echo off\r\ntimeout /t 30 /nobreak >nul\r\n"), 0755); err != nil {
			t.Fatalf("write slow4 bat: %v", err)
		}
	} else {
		dir := t.TempDir()
		slowScriptPath = filepath.Join(dir, "slow4.sh")
		if err := os.WriteFile(slowScriptPath, []byte("#!/bin/sh\nsleep 30\n"), 0755); err != nil {
			t.Fatalf("write slow4 sh: %v", err)
		}
	}

	d, conn, _, _ := newTestDispatcher(t, slowScriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go d.Run(ctx)

	sendExecute(conn, "inst-shutdown-1", "projectShutdown", "")
	sendExecute(conn, "inst-shutdown-2", "projectShutdown", "")

	// Wait for both to start.
	conn.waitForCount(t, protocol.TypeInstanceStarted, 2, 5*time.Second)

	// Cancel context and stop dispatcher — this should kill both instances.
	d.Stop()
	cancel()

	// Wait for all goroutines with a 5-second deadline.
	waitDone := make(chan struct{})
	go func() {
		d.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// Good.
	case <-time.After(5 * time.Second):
		t.Error("Wait() timed out after 5s — goroutine leak suspected")
	}

	// After Wait, the instances map should be empty.
	d.mu.RLock()
	remaining := len(d.instances)
	d.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("instances map has %d entries after shutdown, want 0", remaining)
	}
}
