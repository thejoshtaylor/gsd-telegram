---
phase: 09-channel-auth
plan: "01"
subsystem: security/middleware
tags: [auth, channel, security, middleware, cache]
dependency_graph:
  requires: []
  provides: [ChannelAuthCache, channel-post-auth, echo-filter]
  affects: [internal/bot/middleware.go, internal/security/channel_auth.go]
tech_stack:
  added: [sync.Map TTL cache]
  patterns: [additive branch middleware, TDD red-green, admin-lookup caching]
key_files:
  created:
    - internal/security/channel_auth.go
    - internal/security/channel_auth_test.go
  modified:
    - internal/bot/middleware.go
    - internal/bot/middleware_test.go
decisions:
  - "echo-filter-first: Echo filter runs before channel auth check — bot's own posts in authorized channels are still dropped"
  - "additive-branch: AuthChecker interface unchanged; channel auth is a fallback branch after the existing user-ID check fails"
  - "cache-ttl-15m: 15-minute TTL for channel admin cache per user decision"
  - "log-debug-not-audit: Echo-filtered messages use log.Debug, not audit log — they're expected noise, not security events"
metrics:
  duration: "~20 minutes"
  completed: "2026-03-20"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 2
---

# Phase 09 Plan 01: Channel Auth Summary

**One-liner:** Echo-filter guard and admin-based channel authorization via TTL-cached GetChatAdministrators lookup in auth middleware.

## What Was Built

### Task 1: ChannelAuthCache (internal/security/channel_auth.go)

A thread-safe TTL cache for channel authorization results using `sync.Map`. The cache avoids repeated `GetChatAdministrators` API calls on every channel post.

- `NewChannelAuthCache(ttl time.Duration) *ChannelAuthCache` — constructor
- `Lookup(channelID int64) (authorized bool, hit bool)` — returns cached result with expiry check; expired entries are deleted inline
- `Store(channelID int64, authorized bool)` — stores result with TTL

Five cache tests: miss, hit-authorized, hit-unauthorized, expiry (50ms TTL + 100ms sleep), multi-channel independence.

**Commit:** `0a13b08`

### Task 2: Echo filter and channel auth branch (internal/bot/middleware.go)

Updated `authMiddlewareWith` with two new parameters and two new execution paths:

**New signature:**
```go
func authMiddlewareWith(checker AuthChecker, channelAuth ChannelAuthFn, auditLog *audit.Logger, next ext.Handler) ext.Handler
```

**Echo filter (AUTH-02) — runs FIRST:**
- If `sender.User.IsBot && sender.User.Id == tgBot.Id` → drop (bot's own reflected post)
- If `sender.IsLinkedChannel()` → drop (automatic forward from linked channel)
- Both use `log.Debug` — not the audit log — since they are expected events, not security violations

**Channel auth fallback (AUTH-01) — runs after user-ID check fails:**
- If `sender.IsChannelPost() && channelAuth != nil` → call `channelAuth(tgBot, channelID)`
- `channelAuth` checks the `ChannelAuthCache`; on miss, calls `GetChatAdministrators` and caches the result
- If any admin's user ID matches `AllowedUsers` → authorized; else → rejected

**`Bot.authMiddleware` updated** to create the 15-minute cache and wire the `channelAuthFn` closure.

Five new tests: echo filter, linked-channel filter, channel authorized, channel unauthorized, echo-before-channel-auth ordering (channelAuth panics if reached).

**Commit:** `110a618`

## Success Criteria Met

- AUTH-01: Channel posts from channels with an authorized admin in AllowedUsers pass auth middleware
- AUTH-02: Bot's own reflected channel posts and linked-channel auto-forwards are silently dropped
- No regression: DM and group messages from authorized users continue to work
- AuthChecker interface unchanged (additive branch)
- Cache TTL is 15 minutes
- Echo filter runs before auth check
- All tests pass (`go test ./...` — 9 packages, all green)
- `go vet ./...` — clean
- `go build ./...` — compiles cleanly

## Deviations from Plan

None — plan executed exactly as written.

## Operator Note

After this code ships, the channel's numeric chat ID does NOT need to be in `TELEGRAM_ALLOWED_USERS`. The bot looks up channel admins via `GetChatAdministrators` and checks if any of them are in the allowlist. The allowlist should contain the **user** IDs of authorized humans, not channel IDs.

## Self-Check: PASSED

Files verified:
- internal/security/channel_auth.go — FOUND
- internal/security/channel_auth_test.go — FOUND
- internal/bot/middleware.go — FOUND (contains ChannelAuthFn, IsChannelPost, IsLinkedChannel, echo filter)
- internal/bot/middleware_test.go — FOUND (contains TestMiddlewareAuthEchoFilter)

Commits verified:
- 0a13b08 — FOUND (ChannelAuthCache)
- 110a618 — FOUND (middleware echo filter + channel auth branch)

Full test suite: PASSED (9 packages, all green)
Build: PASSED
go vet: PASSED
