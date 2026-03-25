---
phase: 16-instance-lifecycle-fixes
verified: 2026-03-25T00:00:00Z
status: passed
score: 4/4 must-haves verified
re_verification: false
---

# Phase 16: Instance Lifecycle Fixes Verification Report

**Phase Goal:** Instance finish events carry real exit codes and session IDs so the server can track instance outcomes and resume sessions
**Verified:** 2026-03-25
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | InstanceFinished carries real OS exit code from exec.ExitError, not hardcoded 0 | VERIFIED | `errors.As(streamErr, &exitErr)` at dispatcher.go:259; `exitCode = exitErr.ExitCode()` at line 260; grep confirms no `ExitCode: 0` literal remains |
| 2 | InstanceFinished carries SessionID populated from proc.SessionID() | VERIFIED | `SessionID: proc.SessionID()` in InstanceFinished emission at dispatcher.go:277 |
| 3 | SessionID field uses omitempty — absent from JSON when empty string | VERIFIED | `SessionID string \`json:"session_id,omitempty"\`` at messages.go:146; `TestInstanceFinishedSessionIDOmitempty` passes |
| 4 | Exit code is -1 when process killed by signal, 0 for clean exit, positive for CLI error | VERIFIED | `ExitCode()` from `*exec.ExitError` returns -1 for signal kills per Go stdlib; documented in messages.go:143-144 and protocol-spec.md:211 |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/protocol/messages.go` | InstanceFinished struct with SessionID field | VERIFIED | Line 146: `SessionID string \`json:"session_id,omitempty"\``; struct updated with doc comment at lines 135-138 |
| `internal/dispatch/dispatcher.go` | Real exit code extraction and SessionID population | VERIFIED | `errors.As(streamErr, &exitErr)` at line 259; `SessionID: proc.SessionID()` at line 277; imports include `"errors"` and `"os/exec"` |
| `internal/dispatch/dispatcher_test.go` | Tests for exit code and session ID in InstanceFinished | VERIFIED | `TestInstanceFinishedExitCodeAndSessionID` and `TestInstanceFinishedNonZeroExitCode` present and passing |
| `internal/protocol/messages_test.go` | JSON round-trip test for SessionID omitempty | VERIFIED | `instance_finished` case in `TestEnvelopeRoundTrip` (line 153-173) tests SessionID="sess-fin-1"; `TestInstanceFinishedSessionIDOmitempty` (line 304-316) verifies absent when empty |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/dispatch/dispatcher.go` | `internal/protocol/messages.go` | InstanceFinished struct with SessionID and ExitCode fields | VERIFIED | `protocol.InstanceFinished{` at line 274 with both fields populated |
| `internal/dispatch/dispatcher.go` | `proc.SessionID()` | SessionID populated from process after Stream() returns | VERIFIED | `proc.SessionID()` called at lines 247 and 277; result flows into InstanceFinished.SessionID |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `internal/dispatch/dispatcher.go` | `exitCode` | `exitErr.ExitCode()` from `errors.As(streamErr, &exitErr)` | Yes — real OS exit code from cmd.Wait via wrapped error | FLOWING |
| `internal/dispatch/dispatcher.go` | `SessionID` | `proc.SessionID()` — parsed from NDJSON `session_id` field in Claude output | Yes — populated from actual subprocess NDJSON stream | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| TestEnvelopeRoundTrip/instance_finished passes with SessionID | `go test ./internal/protocol/... -run TestEnvelopeRoundTrip -v -count=1` | PASS | VERIFIED |
| TestInstanceFinishedSessionIDOmitempty passes | `go test ./internal/protocol/... -run TestInstanceFinishedSessionIDOmitempty -v -count=1` | PASS | VERIFIED |
| TestInstanceFinishedExitCodeAndSessionID passes | `go test ./internal/dispatch/... -run TestInstanceFinishedExitCodeAndSessionID -v -count=1` | PASS (0.19s) | VERIFIED |
| TestInstanceFinishedNonZeroExitCode passes | `go test ./internal/dispatch/... -run TestInstanceFinishedNonZeroExitCode -v -count=1` | PASS (8.01s) | VERIFIED |
| Project builds cleanly | `go build ./...` | No output (success) | VERIFIED |
| No hardcoded ExitCode: 0 in dispatcher | `grep 'ExitCode:.*0' internal/dispatch/dispatcher.go` | No matches | VERIFIED |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| INST-04 | 16-01-PLAN.md | Node sends lifecycle events: `instance_started`, `instance_finished`, `instance_error` | SATISFIED | Phase 16 adds SessionID and real exit code to `instance_finished`; the event type itself was established in Phase 13 and extended here with the fields needed for server-side tracking |
| INST-07 | 16-01-PLAN.md | Instances use `--resume SESSION_ID` to maintain persistent Claude sessions across restarts | SATISFIED | `InstanceFinished.SessionID` populated from `proc.SessionID()` gives the server the session ID to pass in future execute commands; `claude.BuildArgs` adds `--resume` when SessionID is non-empty (tested in `TestResumeSession`) |

No orphaned requirements — both INST-04 and INST-07 are declared in the PLAN frontmatter and both are accounted for by implementation evidence.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

No TODO/FIXME, no placeholder returns, no hardcoded empty values, no stub patterns detected in modified files.

### Human Verification Required

None. All behaviors are verifiable programmatically via tests and static analysis.

### Gaps Summary

No gaps. All four must-have truths are verified. Both requirement IDs (INST-04, INST-07) are satisfied. All artifacts exist, are substantive, are wired, and have real data flowing through them. All tests pass. The project builds cleanly.

Key verifications:
- `internal/protocol/messages.go:146` — `SessionID string \`json:"session_id,omitempty"\`` is present in `InstanceFinished`
- `internal/dispatch/dispatcher.go:259` — `errors.As(streamErr, &exitErr)` extracts real exit code
- `internal/dispatch/dispatcher.go:277` — `SessionID: proc.SessionID()` populates from real subprocess output
- `internal/dispatch/dispatcher.go` — no `ExitCode: 0` hardcoded literal anywhere
- `docs/protocol-spec.md:211-212` — `instance_finished` field table documents both `exit_code` semantics and `session_id`
- `docs/server-spec.md:82` — `session_id` described as "authoritative final session ID" from `instance_finished`

---

_Verified: 2026-03-25_
_Verifier: Claude (gsd-verifier)_
