---
phase: 13-dispatch-instance-management-and-node-lifecycle
plan: "03"
subsystem: main
tags: [main, wiring, graceful-shutdown, integration, node-lifecycle]
dependency_graph:
  requires: ["13-02", "internal/dispatch", "internal/connection", "internal/audit", "internal/security", "internal/config"]
  provides: ["main.go - full node binary"]
  affects: []
tech_stack:
  added: []
  patterns: ["context cancellation for shutdown propagation", "10s drain timeout with WaitGroup", "ordered shutdown: cancel -> Stop -> Wait -> connMgr.Stop"]
key_files:
  created: []
  modified:
    - main.go
decisions:
  - "ConnectionManager.Start() before Dispatcher.Run() — recv channel must be ready before dispatcher reads from it"
  - "Shutdown order: cancel context -> Stop dispatcher -> Wait (10s timeout) -> Stop ConnectionManager — ensures disconnect frame sent after instances drain"
  - "nodeLog carries node_id field via zerolog sub-logger — all log lines include node identity (NODE-05)"
  - "Rate limiter always created regardless of RateLimitEnabled — dispatcher checks the flag internally"
metrics:
  duration_seconds: 112
  completed_date: "2026-03-21"
  tasks_completed: 2
  tasks_total: 2
  files_created: 0
  files_modified: 1
---

# Phase 13 Plan 03: Node Wiring and Integration Summary

**One-liner:** main.go fully wired with ConnectionManager, Dispatcher, audit logger, and rate limiter, with 3-phase ordered graceful shutdown (cancel -> drain 10s -> disconnect).

## What Was Built

`main.go` now runs the complete gsd node binary. It replaces the Phase 13 TODO placeholder with full component wiring and graceful shutdown.

### Startup Sequence

1. Load `Config` (working dir, CLI path, rate limit settings, audit path)
2. Load `NodeConfig` (server URL, server token, heartbeat interval, derived node ID)
3. Create nodeLog sub-logger with `node_id` field on every line
4. Ensure data directory exists
5. Open audit logger at `cfg.AuditLogPath`
6. Create `ProjectRateLimiter` from config rate limit settings
7. Set up OS signal handling for `SIGINT`/`SIGTERM`
8. Create base context with cancel
9. Create and Start `ConnectionManager` (dial loop begins, recv channel ready)
10. Create `Dispatcher` and launch `dispatcher.Run(ctx)` in goroutine
11. Block on signal channel

### Shutdown Sequence

**Phase 1:** `cancel()` — cancels context, propagates to all instance goroutines (Claude CLI subprocesses will be killed via context cancellation).

**Phase 2:** `dispatcher.Stop()` signals the Run loop to exit. A goroutine calls `dispatcher.Wait()` (WaitGroup for all instance goroutines) with a 10-second timeout. All active instances drain or are force-killed.

**Phase 3:** `connMgr.Stop()` — sends NodeDisconnect frame, closes WebSocket connection. This is last because the disconnect frame must be sent while the connection is still open and after instances have had a chance to emit their InstanceFinished/InstanceError events.

### Integration Verification (Task 2)

Full project build and test results:

| Package | Status |
|---------|--------|
| `github.com/user/gsd-tele-go` (main) | build OK, no test files |
| `internal/audit` | PASS |
| `internal/claude` | PASS |
| `internal/config` | PASS |
| `internal/connection` | PASS |
| `internal/dispatch` | PASS |
| `internal/protocol` | PASS |
| `internal/security` | PASS |
| `internal/session` | PASS |

`go build ./...` — OK
`go vet ./...` — OK
`go test ./... -timeout 120s` — all 8 packages pass

## Deviations from Plan

**1. [Rule 3 - Environment Constraint] -race flag requires CGO/GCC not available on this Windows build**

- **Found during:** Task 2 verification
- **Issue:** `go test -race` requires CGO which requires GCC. GCC is not in PATH on this Windows system.
- **Fix:** Ran tests without -race flag. All 8 packages pass cleanly. This is consistent with the same finding in 13-02.
- **Impact:** Data race detection requires GCC in environment. Tests verify correctness and goroutine safety via goleak (in dispatch tests). Race detection available in CI with GCC.

None other — plan executed as written.

## Self-Check

### Files Modified
- `main.go` — FOUND

### Commits
- `03a7707` — feat(13-03): wire main.go with full startup and graceful shutdown — FOUND

## Self-Check: PASSED
