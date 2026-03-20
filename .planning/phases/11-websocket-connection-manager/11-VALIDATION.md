---
phase: 11
slug: websocket-connection-manager
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-20
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — standard Go test runner |
| **Quick run command** | `go test ./internal/connection/... -count=1` |
| **Full suite command** | `go test -race ./... -count=1` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/connection/... -count=1`
- **After every plan wave:** Run `go test -race ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 11-01-01 | 01 | 0 | PROTO-02 | unit | `go test ./internal/protocol/... -count=1` | ✅ | ⬜ pending |
| 11-02-01 | 02 | 1 | XPORT-01 | unit | `go test ./internal/connection/... -run TestConnect -count=1` | ❌ W0 | ⬜ pending |
| 11-02-02 | 02 | 1 | XPORT-02 | unit | `go test ./internal/connection/... -run TestReconnect -count=1` | ❌ W0 | ⬜ pending |
| 11-02-03 | 02 | 1 | XPORT-03 | unit | `go test ./internal/connection/... -run TestHeartbeat -count=1` | ❌ W0 | ⬜ pending |
| 11-02-04 | 02 | 1 | XPORT-04 | unit | `go test ./internal/connection/... -run TestRegistration -count=1` | ❌ W0 | ⬜ pending |
| 11-02-05 | 02 | 1 | XPORT-05 | unit | `go test ./internal/connection/... -run TestDisconnect -count=1` | ❌ W0 | ⬜ pending |
| 11-02-06 | 02 | 1 | XPORT-06 | stress | `go test -race ./internal/connection/... -run TestConcurrent -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/connection/manager_test.go` — test stubs for all connection manager behaviors
- [ ] `internal/connection/mock_server_test.go` — TLS mock WebSocket server helper for tests

*Existing test infrastructure covers Go test runner — no framework install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real network drop recovery | XPORT-02 | Requires actual network disruption | Disconnect network cable during active connection, verify reconnect within 30s |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
