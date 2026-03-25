---
phase: 15-project-config-and-registration
plan: 01
subsystem: config
tags: [go, node-config, env-vars, websocket, registration, projects]

# Dependency graph
requires:
  - phase: 10-protocol-message-types
    provides: "NodeRegister struct with Projects []string field on wire protocol"
  - phase: 11-websocket-connection-manager
    provides: "ConnectionManager.sendRegister method that builds NodeRegister frame"
provides:
  - NodeConfig.Projects field populated from PROJECTS env var (comma-separated, whitespace-trimmed)
  - sendRegister uses m.cfg.Projects instead of hard-coded empty slice
  - Registration frame carries real project list from operator config
  - .env.example updated for v1.2 (Telegram vars removed, node vars documented)
affects:
  - server-registration-handler
  - operator-deployment-docs

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "PROJECTS env var: comma-split + TrimSpace + skip empty tokens"
    - "Non-nil empty slice default: cfg.Projects = []string{} before conditional append"

key-files:
  created: []
  modified:
    - internal/config/node_config.go
    - internal/config/node_config_test.go
    - internal/connection/register.go
    - internal/connection/manager_test.go
    - .env.example

key-decisions:
  - "Non-nil empty slice default for Projects ensures JSON serializes as [] not null"
  - "TestRegisterOnConnect channel upgraded from chan string to chan protocol.NodeRegister to assert full payload including Projects"

patterns-established:
  - "Env var parsing pattern: initialize to non-nil default, conditionally populate from env string"

requirements-completed: [PROTO-01, NODE-02]

# Metrics
duration: 6min
completed: 2026-03-25
---

# Phase 15 Plan 01: Project Config and Registration Summary

**NodeConfig.Projects field reads PROJECTS env var and flows through sendRegister so the NodeRegister frame carries the real project list instead of a hard-coded empty slice**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-03-25T07:03:00Z
- **Completed:** 2026-03-25T07:09:26Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Added `Projects []string` to `NodeConfig` with PROJECTS env var parsing (comma-split, TrimSpace, skip empty tokens)
- Empty/unset PROJECTS produces `[]string{}` (non-nil) so JSON serializes as `[]` not `null`
- Changed `sendRegister` from hard-coded `[]string{}` to `m.cfg.Projects`
- TestRegisterOnConnect now sets projects on config, receives full `protocol.NodeRegister` and asserts both NodeID and Projects round-trip
- Replaced .env.example Telegram-era contents with v1.2 node configuration including PROJECTS

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Projects field to NodeConfig and parse PROJECTS env var** - `b7d41ea` (feat)
2. **Task 2: Wire Projects through sendRegister and update .env.example** - `991e177` (feat)

**Plan metadata:** (docs commit follows)

_Note: Task 1 followed TDD pattern: RED (failing tests), GREEN (implementation), all tests pass._

## Files Created/Modified
- `internal/config/node_config.go` - Added Projects []string field and PROJECTS env var parsing block
- `internal/config/node_config_test.go` - Added TestLoadNodeConfigProjects, TestLoadNodeConfigProjectsEmpty, TestLoadNodeConfigProjectsSingleItem
- `internal/connection/register.go` - Changed Projects: []string{} to Projects: m.cfg.Projects
- `internal/connection/manager_test.go` - Updated newTestConfig with Projects field, TestRegisterOnConnect upgraded to verify projects round-trip
- `.env.example` - Full replacement: Telegram vars removed, node vars (SERVER_URL, SERVER_TOKEN, PROJECTS, etc.) documented

## Decisions Made
- Non-nil empty slice default for Projects: `cfg.Projects = []string{}` before optional append ensures JSON serializes as `[]` not `null` — matches RunningInstances pattern from Phase 10
- TestRegisterOnConnect upgraded from `chan string` (NodeID only) to `chan protocol.NodeRegister` (full struct) to verify the full Projects round-trip; this is a stronger test with no behavioral change to production code

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

Pre-existing issues discovered (out of scope, not fixed):
- `TestValidatePathWindowsTraversal` in `internal/security` fails on macOS — Windows-specific path normalization test, pre-existed before this plan
- Data race in `TestKillOneInstance` in `internal/dispatch` under `-race` flag — shared `bytes.Buffer` zerolog writer across goroutines, pre-existed before this plan

Both documented for future attention. Neither is caused by or related to this plan's changes.

## Known Stubs

None — all data paths are wired. PROJECTS env var flows from config to NodeRegister wire frame.

## Next Phase Readiness
- NodeConfig.Projects is populated and flows through registration — server can now route commands based on project list
- .env.example is clean for v1.2 operator deployment
- Phase 15 is complete (only 1 plan in phase)

---
*Phase: 15-project-config-and-registration*
*Completed: 2026-03-25*

## Self-Check: PASSED

- FOUND: internal/config/node_config.go
- FOUND: internal/config/node_config_test.go
- FOUND: internal/connection/register.go
- FOUND: internal/connection/manager_test.go
- FOUND: .env.example
- FOUND: .planning/phases/15-project-config-and-registration/15-01-SUMMARY.md
- FOUND commit: b7d41ea (Task 1)
- FOUND commit: 991e177 (Task 2)
