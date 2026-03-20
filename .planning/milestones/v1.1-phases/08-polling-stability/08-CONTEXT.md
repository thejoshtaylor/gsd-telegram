# Phase 8: Polling Stability - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix the long-polling timeout race: the HTTP client timeout must exceed the getUpdates long-poll Timeout so the HTTP layer never cancels before Telegram responds.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — pure infrastructure phase.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/bot/bot.go` — polling setup at line 121-129
- `gotgbot.GetUpdatesOpts{Timeout: 10}` — current 10-second long-poll window
- No `RequestOpts` currently set on GetUpdatesOpts

### Established Patterns
- gotgbot/v2 uses `RequestOpts.Timeout` per-request to override HTTP client timeout
- `internal/handlers/helpers.go` uses explicit `http.Client{Timeout: 60*time.Second}` for external calls

### Integration Points
- `bot.go:121` — `b.updater.StartPolling()` call where fix applies
- Must not change default timeouts for sendMessage/editMessage API calls

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
