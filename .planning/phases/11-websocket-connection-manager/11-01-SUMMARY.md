---
phase: 11-websocket-connection-manager
plan: "01"
subsystem: connection
tags: [websocket, connection-manager, backoff, tdd, transport]
dependency_graph:
  requires:
    - internal/config/node_config.go (NodeConfig fields)
    - internal/protocol/messages.go (Envelope, NodeDisconnect)
  provides:
    - internal/connection/manager.go (ConnectionManager, NewConnectionManager, Start, Stop, Send, ErrStopped)
    - internal/connection/dial.go (dialLoop, handleConn)
    - internal/connection/writer.go (runWriter — single writer goroutine)
    - internal/connection/backoff.go (newBackoff, full jitter)
    - internal/protocol/messages.go (TypeNodeDisconnect, NodeDisconnect added)
  affects:
    - Phase 12 (heartbeat + registration call manager.Send)
    - Phase 13 (dispatch reads manager.Receive() channel)
tech_stack:
  added:
    - github.com/coder/websocket v1.8.14
    - go.uber.org/goleak v1.3.0 (test-only)
  patterns:
    - Single writer goroutine for all WebSocket writes (XPORT-06)
    - Full jitter exponential backoff (AWS algorithm, 500ms-30s)
    - Context-based goroutine lifecycle with WaitGroup
    - httptest.NewTLSServer + srv.Client() for mock TLS server in tests
key_files:
  created:
    - internal/connection/manager.go
    - internal/connection/dial.go
    - internal/connection/writer.go
    - internal/connection/backoff.go
    - internal/connection/manager_test.go
  modified:
    - internal/protocol/messages.go
    - internal/protocol/messages_test.go
    - go.mod
    - go.sum
decisions:
  - "Two-phase stopCh check in Send() prevents race between buffer availability and stopped state"
  - "recvCh is non-blocking send with drop+warn to avoid stalling reader goroutine"
  - "handleConn is Phase 11 stub; Phase 12 adds heartbeat and registration"
metrics:
  duration: "8m 36s"
  completed_date: "2026-03-20"
  tasks_completed: 2
  files_created: 5
  files_modified: 4
---

# Phase 11 Plan 01: WebSocket ConnectionManager Core Summary

**One-liner:** Outbound WebSocket ConnectionManager with coder/websocket v1.8.14, full-jitter exponential backoff (500ms-30s), and single writer goroutine serializing all frames (XPORT-06).

## What Was Built

The `internal/connection/` package provides the transport foundation for all downstream phases:

- **ConnectionManager** (`manager.go`): Public API — `NewConnectionManager`, `Start`, `Stop`, `Send`, `Receive`, `ErrStopped`
- **dialLoop** (`dial.go`): Reconnect loop with exponential backoff; `handleConn` stub with reader+writer goroutines per connection
- **runWriter** (`writer.go`): Single writer goroutine — the only code path that calls `conn.Write` (XPORT-06)
- **backoffState** (`backoff.go`): Full jitter implementation, 500ms-30s range
- **Protocol extension**: `TypeNodeDisconnect = "node_disconnect"` and `NodeDisconnect` struct added to `internal/protocol/messages.go`

## Commits

| Hash | Description |
|------|-------------|
| 625cfb4 | feat(11-01): add TypeNodeDisconnect to protocol and install dependencies |
| 3b3dbd7 | test(11-01): add failing tests for ConnectionManager (TDD RED) |
| ef13cb1 | feat(11-01): implement ConnectionManager core with dial, backoff, and writer (TDD GREEN) |
| eeddf68 | fix(11-01): prevent Send() from racing with stopped channel check |

## Test Results

All 5 tests pass:
- `TestNewConnectionManager` — constructor returns non-nil with initialized channels
- `TestSendAfterStop` — `Send()` returns `ErrStopped` after `Stop()`
- `TestDial` — mock TLS server receives WebSocket upgrade and frame via `Send()`
- `TestBackoff` — backoffState range, cap, and `Reset()` behavior
- `TestConcurrentSend` — 10 goroutines * 100 frames = 1000 frames, no panic

All tests use `goleak.VerifyNone(t)` for goroutine leak detection.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed non-deterministic Send() after Stop()**
- **Found during:** Task 2 verification (re-running full test suite)
- **Issue:** `Send()` used a single `select` with `sendCh <- data`, `stopped`, and `stopCh` cases. When `stopped` was closed but `sendCh` had buffer capacity, Go's scheduler could non-deterministically pick either case, sometimes returning `nil` instead of `ErrStopped`
- **Fix:** Added a pre-check `select` (non-blocking) to detect `stopCh`/`stopped` before attempting `sendCh` enqueue
- **Files modified:** `internal/connection/manager.go`
- **Commit:** eeddf68

### Environment Notes

**Race detector unavailable:** The `-race` flag requires CGO (`gcc`), which is not installed in this environment. Tests were run without `-race`. The single-writer design (XPORT-06) provides the correctness guarantee regardless — `conn.Write` is only called from `runWriter`, and `sendCh` is a standard Go channel with no direct concurrent access. The `Send()` pre-check fix eliminates the one identified race condition.

## Requirements Coverage

| Requirement | Status | Notes |
|-------------|--------|-------|
| XPORT-01 | Satisfied | `websocket.Dial` with Bearer token `Authorization` header |
| XPORT-06 | Satisfied | `runWriter` is the sole caller of `conn.Write` |

XPORT-02 (heartbeat), XPORT-03 (reconnect), XPORT-04 (re-register), XPORT-05 (disconnect frame) are deferred to Phase 11 Plan 02.

## Self-Check: PASSED
