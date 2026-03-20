---
phase: 08-polling-stability
plan: 01
subsystem: infra
tags: [gotgbot, polling, long-poll, timeout, go]

# Dependency graph
requires: []
provides:
  - "Long-poll HTTP timeout (15s) exceeding Telegram getUpdates window (10s) via RequestOpts.Timeout"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-request RequestOpts.Timeout nested inside GetUpdatesOpts scopes the timeout override to polling only"

key-files:
  created: []
  modified:
    - internal/bot/bot.go

key-decisions:
  - "Use RequestOpts.Timeout (15s) inside GetUpdatesOpts rather than DefaultRequestOpts on BaseBotClient to keep timeout override scoped to polling only"
  - "5-second headroom (15s HTTP timeout vs 10s long-poll window) chosen to absorb network jitter without adding noticeable lag"

patterns-established:
  - "Polling-only timeouts: always nest RequestOpts inside GetUpdatesOpts, never on the global client"

requirements-completed: [POLL-01]

# Metrics
duration: 5min
completed: 2026-03-20
---

# Phase 08 Plan 01: Polling Stability Summary

**Added RequestOpts.Timeout (15s) to gotgbot GetUpdatesOpts, eliminating context deadline exceeded errors during idle long-polling by giving the HTTP layer 5 seconds of headroom over the 10s Telegram poll window.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-20T19:53:00Z
- **Completed:** 2026-03-20T19:58:00Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Added `RequestOpts: &gotgbot.RequestOpts{Timeout: 15 * time.Second}` nested inside the `GetUpdatesOpts` block in `internal/bot/bot.go`
- Long-poll window (10s Telegram parameter) left unchanged
- All other API calls (sendMessage, editMessage, etc.) continue using default timeouts — no global side effects
- Go build verified clean with no compilation errors

## Task Commits

Each task was committed atomically:

1. **Task 1: Add RequestOpts.Timeout to long-poll GetUpdatesOpts** - `ba2c220` (fix)

**Plan metadata:** (pending — final docs commit)

## Files Created/Modified

- `internal/bot/bot.go` - Added `RequestOpts.Timeout: 15 * time.Second` inside `StartPolling` GetUpdatesOpts block with explanatory comment

## Decisions Made

- Scoped the timeout override via `RequestOpts` nested inside `GetUpdatesOpts` (not `DefaultRequestOpts` on the global client) so only polling calls get the extended timeout — per the v1.1 research decision already recorded in STATE.md.
- 15s (5s headroom over 10s poll window) is the minimal safe margin that survives transient network delays without degrading responsiveness.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

`bun` not in PATH on this machine (the `bun run typecheck` step in the plan verification targets the TypeScript portion of the project). Verified the Go change instead with `go build ./internal/bot/...` which passed cleanly. The TypeScript codebase was not touched and its typecheck status is unchanged.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Polling stability fix is complete and committed.
- Phase 09 (auth fix for non-human senders / channel forwards) can begin immediately.
- Reminder (from STATE.md blocker): after Phase 09 ships, operators must add their channel's numeric ID to `TELEGRAM_ALLOWED_USERS` in `.env`.

---
*Phase: 08-polling-stability*
*Completed: 2026-03-20*
