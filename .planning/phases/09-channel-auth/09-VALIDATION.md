---
phase: 9
slug: channel-auth
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-20
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test tooling |
| **Quick run command** | `go test ./internal/security/... ./internal/bot/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/security/... ./internal/bot/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | AUTH-01 | unit | `go test ./internal/security/ -run TestChannelAuthCache` | ❌ W0 | ⬜ pending |
| 09-01-02 | 01 | 1 | AUTH-02 | unit | `go test ./internal/bot/ -run TestEchoFilter` | ❌ W0 | ⬜ pending |
| 09-01-03 | 01 | 1 | AUTH-01 | unit | `go test ./internal/bot/ -run TestAuthMiddleware` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/security/channel_auth_test.go` — tests for ChannelAuthCache
- [ ] `internal/bot/middleware_test.go` — extend existing tests for channel and echo scenarios

*Existing test infrastructure covers Go test framework.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Channel post accepted in authorized channel | AUTH-01 | Requires live Telegram channel + bot | Send message in authorized channel, verify bot responds |
| Unauthorized channel rejected | AUTH-01 | Requires unauthorized channel setup | Send message in non-authorized channel, verify rejection |
| Bot echo not re-processed | AUTH-02 | Requires live bot posting in channel | Bot sends response in channel, verify no echo loop |
| DM auth unchanged | AUTH-01 | Regression check | Send DM from authorized user, verify normal behavior |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
