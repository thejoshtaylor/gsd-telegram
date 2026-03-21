---
phase: 12-telegram-removal-and-session-migration
plan: 01
subsystem: infra
tags: [go, config, cleanup, telegram-removal, typescript-removal]

# Dependency graph
requires:
  - phase: 11-websocket-connection-manager
    provides: ConnectionManager and WebSocket infrastructure this build targets
provides:
  - Clean Go codebase with no Telegram/gotgbot/openai imports
  - Config struct with only node-relevant fields (WorkingDir, ClaudeCLIPath, AllowedPaths, RateLimitEnabled/Requests/Window, AuditLogPath, DataDir, SafetyPrompt)
  - Minimal main.go placeholder ready for Phase 13 wiring
affects:
  - 12-02-session-migration
  - 13-dispatch-instance-management

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Config without Telegram fields — no TELEGRAM_BOT_TOKEN or TELEGRAM_ALLOWED_USERS required"
    - "main.go as minimal placeholder with TODO(phase-13) marker for ConnectionManager wiring"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - main.go

key-decisions:
  - "Telegram constant block removed entirely — TelegramMessageLimit, TelegramSafeLimit, StreamingThrottleMs, ButtonLabelMaxLength, QueryTimeoutMs are node-irrelevant"
  - "Default audit log renamed from claude-telegram-audit.log to gsd-node-audit.log"
  - "BuildSafetyPrompt header updated from TELEGRAM BOT to GSD NODE, removed running-via-Telegram disclaimer"
  - "config_test.go rewritten — removed all TELEGRAM_BOT_TOKEN/TELEGRAM_ALLOWED_USERS test cases, removed PdfToTextPath tests"

patterns-established:
  - "main.go TODO(phase-13): convention for marking Phase 13 wiring points"

requirements-completed:
  - CLEAN-01
  - CLEAN-02
  - CLEAN-03

# Metrics
duration: 25min
completed: 2026-03-20
---

# Phase 12 Plan 01: Telegram Removal and Session Migration Summary

**Config struct stripped to node-only fields, TypeScript artifacts deleted, gotgbot removed, main.go rewritten as minimal placeholder with TODO(phase-13) marker**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-03-20T17:30:00Z
- **Completed:** 2026-03-20T17:55:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Removed all Telegram-specific fields from Config struct (TelegramToken, AllowedUsers, OpenAIAPIKey, PdfToTextPath)
- Removed Telegram-specific constants (TelegramMessageLimit, TelegramSafeLimit, StreamingThrottleMs, ButtonLabelMaxLength, QueryTimeoutMs)
- Rewrote main.go to compile without internal/bot, log startup info, and include TODO(phase-13) placeholder
- Renamed default audit log path from claude-telegram-audit.log to gsd-node-audit.log
- All 7 packages build and test pass: audit, claude, config, connection, protocol, security, session
- internal/ contains only: audit, claude, config, connection, protocol, security, session

## Task Commits

Each task was committed atomically:

1. **Task 1: Delete Telegram packages, TypeScript files, and strip dependencies** - `3f19f9f` (feat by parallel agent 12-02 — TypeScript and package deletions already committed)
2. **Task 2: Strip Telegram config fields and rewrite main.go** - `3a3dce3` (feat)

## Files Created/Modified

- `internal/config/config.go` - Removed Telegram fields, constants, Load() requirements; renamed audit log; updated BuildSafetyPrompt
- `internal/config/config_test.go` - Rewritten to test node-only config (removed all Telegram-specific test cases)
- `main.go` - Removed internal/bot import; minimal entry point with TODO(phase-13) placeholder

## Decisions Made

- Telegram constant block removed entirely — none of TelegramMessageLimit, TelegramSafeLimit, StreamingThrottleMs, ButtonLabelMaxLength, QueryTimeoutMs are needed for the node
- BuildSafetyPrompt header updated: "CRITICAL SAFETY RULES FOR TELEGRAM BOT" → "CRITICAL SAFETY RULES FOR GSD NODE"
- Removed "You are running via Telegram..." disclaimer from safety prompt footer
- Default audit log: claude-telegram-audit.log → gsd-node-audit.log

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] config_test.go referenced removed Telegram fields and constants**

- **Found during:** Task 2 (strip Telegram config fields)
- **Issue:** config_test.go tested TelegramToken, AllowedUsers, PdfToTextPath fields and TelegramMessageLimit/TelegramSafeLimit/StreamingThrottleMs constants that were all removed from config.go. Tests would fail to compile.
- **Fix:** Rewrote config_test.go — removed TestLoadConfigMissingToken, TestLoadConfigMissingUsers, TestConstants (for Telegram constants), TestLoadConfig_PdfToTextPath, TestLoadConfig_PdfToTextPathMissing; updated TestLoadConfig to not set TELEGRAM_BOT_TOKEN; updated TestConstants to only test MaxSessionHistory and SessionQueueSize.
- **Files modified:** internal/config/config_test.go
- **Verification:** `go test ./internal/config/...` passes
- **Committed in:** 3a3dce3 (Task 2 commit)

**2. [Context] TypeScript and package.json/bun.lock/src/ deletions were already committed by parallel agent 12-02**

- Parallel agent running plan 12-02 committed deletion of all TypeScript artifacts (src/, bun.lock, package.json, package-lock.json, tsconfig.json, vitest.config.ts, mcp-config.example.ts) as part of commit 3f19f9f.
- The Go internal directories (internal/bot, internal/handlers, internal/formatting, internal/project) referenced in the plan never existed in the Go codebase — the plan's files_modified listed TypeScript/old-phase file paths.
- The gotgbot dependency in go.mod was never present in the Go codebase HEAD.
- Task 1's work was therefore already complete prior to this plan's execution; no additional commit needed for Task 1.

---

**Total deviations:** 1 auto-fixed (bug in tests), 1 context note (parallel agent overlap)
**Impact on plan:** Auto-fix essential for compilation. Parallel agent overlap had no negative impact.

## Issues Encountered

- go mod tidy initially failed because main.go still imported internal/bot — resolved by rewriting main.go (Task 2) before running tidy.
- main.go had an unused `ctx` variable after initial rewrite — fixed by using blank identifier `_, cancel := context.WithCancel(...)` with comment explaining Phase 13 wiring.

## User Setup Required

None - no external service configuration required. The node no longer requires TELEGRAM_BOT_TOKEN or TELEGRAM_ALLOWED_USERS.

## Next Phase Readiness

- Phase 12-02 (session migration): already in progress (parallel agent committed string-keyed SessionStore)
- Phase 13 (dispatch/instance management): main.go has TODO(phase-13) marker; config.Load() has no Telegram requirements
- internal/ is clean — only packages needed for WebSocket node remain

---
*Phase: 12-telegram-removal-and-session-migration*
*Completed: 2026-03-20*
