---
phase: quick
plan: 260324-wld
subsystem: config,dispatch
tags: [security, websocket, validation]
dependency_graph:
  requires: []
  provides: [wss-scheme-enforcement]
  affects: [internal/config, internal/dispatch]
tech_stack:
  added: []
  patterns: [input-validation-at-boundary]
key_files:
  created: []
  modified:
    - internal/config/node_config.go
    - internal/config/node_config_test.go
    - internal/dispatch/dispatcher_test.go
decisions:
  - LoadNodeConfig rejects non-wss:// URLs at config parse time — plaintext ws:// blocked before any connection attempt
metrics:
  duration: ~4 minutes
  completed: "2026-03-24"
  tasks_completed: 2
  files_modified: 3
---

# Quick Task 260324-wld: Enforce wss:// for all WebSocket connections — Summary

**One-liner:** wss:// scheme validation added to LoadNodeConfig with descriptive error; ws:// test fixture corrected.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add wss:// validation + rejection test | 9839ae2 | internal/config/node_config.go, internal/config/node_config_test.go |
| 2 | Fix dispatcher test fixture | 6536497 | internal/dispatch/dispatcher_test.go |

## Changes Made

### Task 1 — wss:// validation in LoadNodeConfig

Added `strings` import to `node_config.go`. After the empty-URL check, inserted:

```go
if !strings.HasPrefix(cfg.ServerURL, "wss://") {
    return nil, fmt.Errorf("SERVER_URL must use wss:// scheme (got %q) — plaintext ws:// is not allowed", cfg.ServerURL)
}
```

Added `TestLoadNodeConfigRejectsInsecureWS` to `node_config_test.go`:
- Sets SERVER_URL to `ws://example.com/ws`
- Asserts error is non-nil
- Asserts error contains both `"wss://"` and `"ws://"`

### Task 2 — Dispatcher test fixture

Changed line 175 in `dispatcher_test.go` from `"ws://localhost:9999"` to `"wss://localhost:9999"`. The fixture is never dialed in the test (connection manager is a stub); change is safe.

## Verification

```
ok  github.com/user/gsd-tele-go/internal/config       0.493s
ok  github.com/user/gsd-tele-go/internal/dispatch     7.103s
ok  github.com/user/gsd-tele-go/internal/connection   3.498s
```

`grep -rn 'ws://' --include='*.go'` (main repo, excluding worktree): only the error message string in `node_config.go` and the rejection test in `node_config_test.go` remain — both are intentional.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- internal/config/node_config.go: modified (wss:// validation present)
- internal/config/node_config_test.go: modified (rejection test present)
- internal/dispatch/dispatcher_test.go: modified (ws:// fixture corrected)
- Commits 9839ae2 and 6536497 exist on main branch
