---
phase: 11
plan: "02"
subsystem: connection
tags: [websocket, heartbeat, registration, clean-shutdown, reconnect, tdd]
dependency_graph:
  requires:
    - 11-01 (ConnectionManager core with dial, backoff, writer)
    - internal/protocol/messages.go (TypeNodeRegister, TypeNodeDisconnect, NodeRegister, NodeDisconnect)
    - internal/config/node_config.go (HeartbeatIntervalSecs, NodeID)
  provides:
    - internal/connection/heartbeat.go (heartbeat ping goroutine with pong timeout)
    - internal/connection/register.go (sendRegister helper sending NodeRegister as first frame)
    - internal/connection/writer.go (updated with clean shutdown disconnect frame logic)
    - internal/connection/manager.go (updated Stop, SetHeartbeatInterval, generateMsgID)
    - internal/connection/dial.go (updated handleConn and dialLoop with all lifecycle)
  affects:
    - Phase 13 (dispatcher) reads from recvCh, which now gets proper frame ordering
tech_stack:
  added:
    - crypto/rand for generateMsgID (stdlib only, no new deps)
    - runtime.GOOS for Platform field in NodeRegister
  patterns:
    - Priority select: non-blocking stopCh check at writer loop start eliminates race with context cancellation
    - Deferred shutdown: Stop() closes stopCh, waits for m.stopped, then calls cancel() — prevents context cancellation from racing with disconnect frame write
    - Writer-owns-shutdown: writer goroutine is sole owner of disconnect frame send and conn.Close on clean shutdown
    - Three-goroutine handleConn: reader + writer + heartbeat, all sharing connCtx, all in WaitGroup
key_files:
  created:
    - internal/connection/heartbeat.go
    - internal/connection/register.go
  modified:
    - internal/connection/writer.go (added stopCh handling, sendDisconnectFrame, cancel param)
    - internal/connection/manager.go (heartbeatInterval field, SetHeartbeatInterval, generateMsgID, sync.Once, revised Stop)
    - internal/connection/dial.go (sendRegister call, runHeartbeat goroutine, stopCh checks in dialLoop, backoff delay on reconnect)
    - internal/connection/manager_test.go (8 new tests, fixed TestDial for register-first ordering)
decisions:
  - "Writer goroutine owns clean shutdown: sends NodeDisconnect frame then conn.Close() before exiting — ensures disconnect frame is written while connection is healthy"
  - "Stop() defers m.cancel() until after m.stopped: prevents context cancellation race that would close connection before disconnect frame is sent"
  - "Priority select in runWriter: non-blocking stopCh check at loop start ensures stopCh is always handled before connCtx.Done() can fire"
  - "sendDisconnectFrame uses context.Background() with 3s timeout: independent of connCtx which may be cancelled"
  - "m.stopCh checks added to all dialLoop exit points: handles stop signals even when no context cancellation occurs"
metrics:
  duration: "~45 minutes"
  completed: "2026-03-21"
  tasks_completed: 2
  files_created: 2
  files_modified: 5
  tests_added: 8
---

# Phase 11 Plan 02: Heartbeat, Registration, and Clean Shutdown Summary

Complete WebSocket connection lifecycle: heartbeat ping/pong, NodeRegister on connect/reconnect, clean shutdown with NodeDisconnect frame, and reconnect-after-drop with exponential backoff.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Add heartbeat ping and registration on connect | 648399d | heartbeat.go, register.go, manager.go, dial.go |
| 2 | Add clean shutdown with disconnect frame | b1c625e | writer.go, manager.go, dial.go |

## What Was Built

### Task 1: Heartbeat + Registration

**`internal/connection/register.go`** — `sendRegister()` builds a `NodeRegister` envelope with NodeID, Platform (runtime.GOOS), Version, and empty Projects/RunningInstances. Writes directly to `conn` (not via `sendCh`) before any goroutines start, guaranteeing it is the first frame on every connection.

**`internal/connection/heartbeat.go`** — `runHeartbeat()` fires a `conn.Ping()` every `heartbeatInterval`. The ping uses a timeout of 3x the interval (`context.WithTimeout`). On timeout/error, calls `cancel()` to kill `connCtx` and trigger reconnect. Runs as a third goroutine inside `handleConn` alongside reader and writer.

**`manager.go` additions:**
- `heartbeatInterval` field (defaults to `cfg.HeartbeatIntervalSecs * time.Second`)
- `SetHeartbeatInterval(d time.Duration)` for test injection (100ms in tests)
- `generateMsgID()` helper using `crypto/rand` for envelope IDs
- `sync.Once` (`stopOnce`) protecting `stopCh` close

**`dial.go` handleConn updates:**
- Calls `sendRegister` before starting any goroutines
- Adds heartbeat goroutine to `wg.WaitGroup`
- All three goroutines (reader, writer, heartbeat) share `connCtx` and `cancel`

### Task 2: Clean Shutdown + Reconnect-After-Drop

**`writer.go` redesign:** The writer goroutine now owns the clean shutdown sequence. It monitors `m.stopCh` with a priority check at the top of each loop iteration. When `stopCh` fires:
1. Calls `sendDisconnectFrame(conn)` — writes NodeDisconnect envelope with 3s timeout
2. Calls `conn.Close(StatusNormalClosure, "shutdown")` — sends WebSocket close frame
3. Returns (reader goroutine exits when it sees the close frame)

**`manager.go` Stop() redesign:** The critical insight: `Stop()` must NOT call `m.cancel()` before waiting for `m.stopped`. Calling `m.cancel()` immediately would cancel `connCtx`, causing `coder/websocket` to close the connection (documented behavior: "If context passed to Read expires, connection is closed"). This would race with the writer trying to send the disconnect frame.

New sequence:
1. `close(m.stopCh)` — signals writer goroutine
2. `<-m.stopped` — wait for dialLoop to exit (writer → reader → wg → handleConn → dialLoop)
3. `m.cancel()` — cleanup only, no-op at this point

**`dial.go` dialLoop updates:**
- `m.stopCh` added to all exit paths (pre-dial check, dial backoff sleep, post-handleConn check)
- Backoff delay added between reconnect attempts with `stopCh` and `ctx.Done()` escape hatches

## XPORT Requirements Coverage

| Req | Test | Status |
|-----|------|--------|
| XPORT-01: Dial and auth | TestDial (Plan 01) | Satisfied |
| XPORT-02: Heartbeat ping/pong | TestHeartbeatKeepsAlive, TestHeartbeatDeadServer | Satisfied |
| XPORT-03: Reconnect with backoff | TestReconnectAfterDrop, TestBackoff (Plan 01) | Satisfied |
| XPORT-04: NodeRegister on connect | TestRegisterOnConnect, TestRegisterAfterReconnect | Satisfied |
| XPORT-05: Clean shutdown with disconnect | TestCleanShutdown | Satisfied |
| XPORT-06: Concurrent send safety | TestConcurrentSend (Plan 01) | Satisfied |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed race between context cancellation and disconnect frame write**
- **Found during:** Task 2
- **Issue:** `Stop()` was calling `m.cancel()` immediately after `close(m.stopCh)`. The `coder/websocket` library closes the underlying connection when any context passed to `Read` is cancelled. This caused the connection to be closed before the writer could send the disconnect frame.
- **Fix:** Restructured `Stop()` to wait for `m.stopped` BEFORE calling `m.cancel()`. Added priority select in `runWriter` to check `stopCh` first. Moved disconnect frame sending from `handleConn` to `runWriter` (the write happens while connection is still alive).
- **Files modified:** `internal/connection/manager.go`, `internal/connection/writer.go`
- **Commit:** b1c625e

**2. [Rule 1 - Bug] Fixed TestDial failing after NodeRegister was added**
- **Found during:** Task 1
- **Issue:** `TestDial` expected the first frame from the server to be `{"type":"test"}` but NodeRegister is now always sent first.
- **Fix:** Updated `TestDial` to skip the first (register) frame and read the second frame.
- **Files modified:** `internal/connection/manager_test.go`
- **Commit:** 648399d

## Self-Check: PASSED

All created files found on disk. All task commits verified in git log.
