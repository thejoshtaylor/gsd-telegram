---
phase: 13
slug: dispatch-instance-management-and-node-lifecycle
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-21
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — standard Go test runner |
| **Quick run command** | `go test ./internal/dispatch/... ./internal/audit/... ./internal/security/... -count=1` |
| **Full suite command** | `go test -race ./... -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick command
- **After every plan wave:** Run `go test -race ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 20 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 13-W0 | W0 | 0 | INST-07, NODE-06 | unit | `go test ./internal/audit/... ./internal/security/... -count=1` | ✅ | ⬜ pending |
| 13-01-01 | 01 | 1 | PROTO-03, INST-01 | integration | `go test ./internal/dispatch/... -run TestRunACK -count=1` | ❌ W0 | ⬜ pending |
| 13-01-02 | 01 | 1 | INST-02, INST-03 | integration | `go test ./internal/dispatch/... -run TestConcurrent -count=1` | ❌ W0 | ⬜ pending |
| 13-02-01 | 02 | 2 | PROTO-04, INST-04 | integration | `go test ./internal/dispatch/... -run TestKill -count=1` | ❌ W0 | ⬜ pending |
| 13-02-02 | 02 | 2 | PROTO-05, INST-05 | integration | `go test ./internal/dispatch/... -run TestStatus -count=1` | ❌ W0 | ⬜ pending |
| 13-03-01 | 03 | 3 | NODE-03, NODE-04 | integration | `go test -race ./internal/dispatch/... -run TestShutdown -count=1 -timeout 30s` | ❌ W0 | ⬜ pending |
| 13-03-02 | 03 | 3 | NODE-05, INST-06 | integration | `go test ./... -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/audit/log.go` — update Event struct with NodeID, InstanceID, Project, Source fields
- [ ] `internal/security/ratelimit.go` — add ProjectRateLimiter with string keys

*Existing test infrastructure and test runner are already in place.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real Claude CLI subprocess streaming | INST-01 | Requires actual Claude CLI binary | Run node against real server, send execute, verify NDJSON output |
| SIGINT/SIGTERM process cleanup | NODE-03 | Requires OS signal delivery | Send SIGINT to running node with active instances, verify cleanup within 10s |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 20s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
