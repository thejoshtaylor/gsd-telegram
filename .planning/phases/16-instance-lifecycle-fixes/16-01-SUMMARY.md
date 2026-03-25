---
phase: 16-instance-lifecycle-fixes
plan: 01
subsystem: protocol
tags: [go, protocol, dispatch, websocket, instance-lifecycle, exit-code, session-id]

# Dependency graph
requires:
  - phase: 13-dispatch-instance-management-and-node-lifecycle
    provides: dispatcher.go with runInstance and terminal event emission
  - phase: 14-protocol-and-server-spec-documents
    provides: protocol-spec.md and server-spec.md as the docs being updated
provides:
  - InstanceFinished struct with SessionID field (omitempty) in internal/protocol/messages.go
  - Real OS exit code extraction via errors.As + exec.ExitError in dispatcher
  - SessionID population from proc.SessionID() in InstanceFinished emission
  - Tests for exit code, session ID, and omitempty behavior
  - Updated protocol-spec.md and server-spec.md with session_id semantics
affects: [server-implementation, resume-capability, instance-tracking]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "errors.As pattern for ExitError extraction from wrapped stream errors"
    - "omitempty on SessionID in InstanceFinished — absent from JSON when empty string"

key-files:
  created: []
  modified:
    - internal/protocol/messages.go
    - internal/protocol/messages_test.go
    - internal/dispatch/dispatcher.go
    - internal/dispatch/dispatcher_test.go
    - docs/protocol-spec.md
    - docs/server-spec.md

key-decisions:
  - "exit_code extracted via errors.As(streamErr, &exec.ExitError) — handles wrapped errors correctly"
  - "SessionID uses omitempty on InstanceFinished — absent from JSON when empty, consistent with InstanceStarted"
  - "InstanceFinished.SessionID is authoritative final session ID; InstanceStarted.SessionID is the input/resume session ID"
  - "exit_code semantics: 0=clean, -1=signal-killed, positive=CLI error code"

patterns-established:
  - "InstanceFinished carries both ExitCode (real OS value) and SessionID (output session for resume)"

requirements-completed: [INST-04, INST-07]

# Metrics
duration: 15min
completed: 2026-03-25
---

# Phase 16 Plan 01: Instance Lifecycle Fixes Summary

**InstanceFinished now carries real OS exit code via exec.ExitError extraction and SessionID from proc.SessionID() for server-side resume capability**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-25T00:00:00Z
- **Completed:** 2026-03-25T00:15:00Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Added `SessionID string` field with `omitempty` to `InstanceFinished` protocol struct
- Replaced hardcoded `ExitCode: 0` with real exit code extracted via `errors.As(streamErr, &exitErr)` and `exitErr.ExitCode()`
- Populated `SessionID: proc.SessionID()` in the InstanceFinished emission block
- Added three new tests: `TestInstanceFinishedExitCodeAndSessionID`, `TestInstanceFinishedNonZeroExitCode`, `TestInstanceFinishedSessionIDOmitempty`
- Updated protocol-spec.md and server-spec.md to document session_id field and real exit code semantics

## Task Commits

1. **Task 1: Add SessionID to InstanceFinished and extract real exit code** - `95e4758` (feat, TDD)
2. **Task 2: Update protocol and server spec docs** - `8905052` (docs)

## Files Created/Modified

- `internal/protocol/messages.go` - Added SessionID field with omitempty to InstanceFinished struct; updated doc comment
- `internal/protocol/messages_test.go` - Updated instance_finished round-trip case to include SessionID; added TestInstanceFinishedSessionIDOmitempty
- `internal/dispatch/dispatcher.go` - Added errors/os/exec imports; extract real exit code before done.Do; populate SessionID in InstanceFinished emission
- `internal/dispatch/dispatcher_test.go` - Added fmt import; createMockClaudeWithExit helper; TestInstanceFinishedExitCodeAndSessionID; TestInstanceFinishedNonZeroExitCode
- `docs/protocol-spec.md` - Updated section 3.1.5: added session_id to JSON schema, field table, Diagram 2, and Diagram 3; updated exit_code semantics
- `docs/server-spec.md` - Updated session_id description (instance_finished is authoritative); updated instance_finished event table row; updated payload example

## Decisions Made

- exit_code extracted via `errors.As(streamErr, &exec.ExitError)` — handles wrapped errors from cmd.Wait correctly without unwrapping manually
- SessionID uses `omitempty` on InstanceFinished — absent when empty, matching the pattern established by InstanceStarted.SessionID
- InstanceFinished.SessionID is the authoritative final session ID; InstanceStarted.SessionID is the input/resume session ID passed in the execute command

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

The `-race` flag detected pre-existing data races in `TestKillOneInstance` and related tests caused by multiple goroutines writing to a shared `bytes.Buffer` used as the zerolog test logger. This race exists in the codebase before these changes (confirmed by stashing and re-running). The race is in test infrastructure only, not production code.

## Next Phase Readiness

- InstanceFinished now carries real exit codes and session IDs — server can implement resume capability
- Protocol spec and server spec updated — server implementers have accurate documentation
- No blockers for Phase 17 (dead-code-removal-and-test-fixes)

---
*Phase: 16-instance-lifecycle-fixes*
*Completed: 2026-03-25*
