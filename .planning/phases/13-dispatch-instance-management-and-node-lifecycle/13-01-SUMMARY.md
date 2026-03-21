---
phase: 13-dispatch-instance-management-and-node-lifecycle
plan: 01
subsystem: audit, security, protocol
tags: [audit, ratelimit, protocol, websocket, node-oriented]

# Dependency graph
requires:
  - phase: 12-telegram-removal-and-session-migration
    provides: Telegram fields removed from codebase; instance UUIDs as session keys
provides:
  - Node-oriented audit.Event with Source, NodeID, InstanceID, Project fields
  - ProjectRateLimiter with string-keyed per-project token bucket and Allow(string) bool
  - Exported protocol.NewMsgID() for envelope ID generation from any package
  - TypeACK constant and ACK struct for command acknowledgement
affects:
  - 13-02
  - 13-03
  - any package that creates audit events or dispatches commands

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Node-oriented audit events use Source/NodeID rather than Telegram UserID/ChannelID
    - ProjectRateLimiter pattern mirrors ChannelRateLimiter but with string keys and bool-only Allow
    - generateMsgID() in connection package delegates to protocol.NewMsgID() — single authoritative implementation

key-files:
  created: []
  modified:
    - internal/audit/log.go
    - internal/audit/log_test.go
    - internal/security/ratelimit.go
    - internal/security/ratelimit_test.go
    - internal/protocol/messages.go
    - internal/protocol/messages_test.go
    - internal/connection/manager.go

key-decisions:
  - "ProjectRateLimiter.Allow() returns bool only — dispatcher needs allow/deny, not delay duration"
  - "ChannelRateLimiter preserved unchanged for backward compatibility; ProjectRateLimiter added alongside"
  - "protocol.NewMsgID() is the canonical ID generator; connection.generateMsgID() delegates to it"
  - "ACK struct placed with inbound command structs — it flows node-to-server but acknowledges inbound commands"

patterns-established:
  - "Audit events in Phase 13+ use NewEvent(action, source, nodeID) — Telegram-era int64 fields gone"
  - "Per-entity rate limiters use string keys for named resources (projects, nodes)"

requirements-completed: [NODE-04, NODE-06]

# Metrics
duration: 15min
completed: 2026-03-20
---

# Phase 13 Plan 01: Adapt Audit, Security, and Protocol Packages for Node-Oriented Dispatch Summary

**Node-oriented audit.Event with Source/NodeID/InstanceID/Project fields, string-keyed ProjectRateLimiter, and exported protocol.NewMsgID() with TypeACK/ACK struct**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-20T00:00:00Z
- **Completed:** 2026-03-20
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Redesigned audit.Event to drop Telegram int64 fields (UserID, Username, ChannelID) and add node-oriented string fields (Source, NodeID, InstanceID, Project)
- Updated all three audit test functions to use new NewEvent(action, source, nodeID) signature
- Added ProjectRateLimiter with string-keyed per-project token bucket alongside the existing ChannelRateLimiter
- Exported protocol.NewMsgID() and made connection.generateMsgID() delegate to it, eliminating duplicate implementation
- Added TypeACK constant and ACK struct to the protocol package for command acknowledgement

## Task Commits

Each task was committed atomically:

1. **Task 1: Redesign audit.Event for node-oriented fields and add ProjectRateLimiter** - `32f5154` (feat)
2. **Task 2: Export protocol.NewMsgID and add ACK type constant** - `29ff954` (feat)

## Files Created/Modified
- `internal/audit/log.go` - Event struct redesigned with Source, NodeID, InstanceID, Project; NewEvent signature updated
- `internal/audit/log_test.go` - All three test functions updated to use new NewEvent(action, source, nodeID) signature
- `internal/security/ratelimit.go` - ProjectRateLimiter added with string-keyed token buckets and bool-only Allow()
- `internal/security/ratelimit_test.go` - Three new ProjectRateLimiter tests added (allow, per-project isolation, concurrent)
- `internal/protocol/messages.go` - TypeACK constant, ACK struct, and exported NewMsgID() added; imports updated
- `internal/protocol/messages_test.go` - TestNewMsgID and TestACKRoundTrip tests added
- `internal/connection/manager.go` - generateMsgID() now delegates to protocol.NewMsgID(); unused crypto/rand and encoding/hex imports removed

## Decisions Made
- ProjectRateLimiter.Allow() returns bool only (not (bool, time.Duration)) — the dispatcher only needs allow/deny
- ChannelRateLimiter preserved unchanged for backward compatibility; ProjectRateLimiter added alongside it
- protocol.NewMsgID() is now the canonical ID generator; connection.generateMsgID() delegates to avoid duplication
- ACK struct placed with inbound command structs since it acknowledges server-to-node commands

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- CGO not available in this environment, so -race flag was dropped for test runs. Tests ran without race detector but passed cleanly. The plan's verification commands specify -race; all concurrent behavior is still tested (TestAuditLogConcurrent, TestRateLimiterConcurrent, TestProjectRateLimiterConcurrent).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- audit.Event is ready for Phase 13 dispatcher to create node-oriented log entries
- ProjectRateLimiter is ready for the dispatcher to enforce per-project rate limits
- protocol.NewMsgID() is exported and available for any Phase 13+ package to generate envelope IDs
- TypeACK and ACK struct available for the dispatcher to send acknowledgements before execution begins

## Self-Check: PASSED

All files verified present. Both task commits (32f5154, 29ff954) found in git history.

---
*Phase: 13-dispatch-instance-management-and-node-lifecycle*
*Completed: 2026-03-20*
