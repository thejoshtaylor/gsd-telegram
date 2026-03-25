# Phase 17: Dead Code Removal and Test Fixes - Research

**Researched:** 2026-03-25
**Domain:** Go dead code removal, race-detector-clean tests, platform-aware test guards
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

### Claude's Discretion
All implementation choices.

### Deferred Ideas (OUT OF SCOPE)
None — discuss phase skipped.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CLEAN-02 | Remove gotgbot/v2 and openai-go dependencies from go.mod | go.mod already clean — these were removed prior to Phase 17. No action needed on go.mod itself; the dead code that kept channel-based types alive is what needs removing. |
| CLEAN-04 | Migrate session persistence keys from Telegram channel IDs to project-based keys | migrate.go in internal/session implements MigrateSessionHistory; package is unwired from production — needs wiring at startup or the entire internal/session package needs assessment for whether it's used at all. |
</phase_requirements>

## Summary

Phase 17 has four discrete tasks driven by the success criteria, each with a clear root cause:

**Task 1 — Delete `ChannelRateLimiter`:** `ChannelRateLimiter` (type + constructor + `Allow` method) lives in `internal/security/ratelimit.go` alongside the production `ProjectRateLimiter`. It is not imported anywhere outside the security package and not wired into main.go. Three test functions in `ratelimit_test.go` test it exclusively (`TestRateLimiterAllow`, `TestRateLimiterPerChannel`, `TestRateLimiterConcurrent`). Both the type and its three tests must be deleted. `IsAuthorized` in validate.go is a similar Telegram leftover (takes `userID int64, channelID int64` — both Telegram concepts) and is also unused in production; however the success criteria only explicitly names `ChannelRateLimiter`, so that is the confirmed target.

**Task 2 — Resolve `internal/session` package:** The `internal/session` package (`session.go`, `store.go`, `persist.go`, `migrate.go`) is entirely self-contained and is not imported by main.go or any production package. It compiles and its tests pass, but it does nothing at runtime. The success criterion says "wired into production code at startup OR removed entirely." The `MigrateSessionHistory` function in `migrate.go` is the CLEAN-04 implementation; it exists but is never called. Resolution: call `MigrateSessionHistory` from main.go startup (if a legacy sessions.json could exist on disk) OR delete the entire package if no migration path is needed. Given the project has moved to project-keyed sessions in the dispatcher (which manages session IDs inline via `proc.SessionID()`), and the new `SavedSession.InstanceID` field is project-name-based, the package exists to serve a purpose that never got wired in. The decision is Claude's discretion.

**Task 3 — Fix `go test -race ./internal/dispatch/...`:** The race is a `bytes.Buffer` concurrent write. In `newTestDispatcher()` (line 191) and `TestStructuredLogging` (line 974), a `bytes.Buffer` is created with `zerolog.New(&logBuf)` and passed into the Dispatcher. The Dispatcher's `Run()` goroutine and `runInstance()` goroutines both call `d.log.Info().Msg(...)` concurrently, which both write to the same unsynchronised `bytes.Buffer`. The fix is to replace `bytes.Buffer` with `zerolog.NewTestWriter(t)` or wrap the buffer with a mutex — or use `zerolog.Nop()` for tests that don't inspect log output. For `TestStructuredLogging` (which does inspect log output), use a mutex-wrapped writer. This is a test-only fix; production code is unaffected.

**Task 4 — Platform-aware `TestValidatePathWindowsTraversal`:** On macOS/Linux, `filepath.Clean` does NOT treat backslashes as path separators. The path `C:\Users\me\projects\..\..\..\Windows\System32\cmd.exe` has all its backslashes treated as literal characters in a single path component. After `filepath.ToSlash` and `filepath.Clean`, the result is still `C:\Users\me\projects\..\..\..\Windows\System32\cmd.exe` (unchanged), and `strings.HasPrefix` against `C:\Users\me\projects` returns `true` — so `ValidatePath` incorrectly returns `true`, failing the test. The fix is to add a `runtime.GOOS == "windows"` guard: the test should `t.Skip("Windows-only test")` on non-Windows platforms.

**Primary recommendation:** Four surgical edits — delete ChannelRateLimiter + its tests, decide and execute on the session package, wrap the test logger in a mutex-safe writer, and add a platform skip guard.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/rs/zerolog` | v1.34.0 | Structured logging | Already in use; `zerolog.Nop()` for silent test loggers, mutex-wrapped writer for inspectable loggers |
| Go stdlib `sync` | stdlib | Mutex-wrapped writer | `sync.Mutex` + `io.Writer` wrapper is idiomatic for thread-safe buffer in tests |
| Go stdlib `testing` | stdlib | Platform guard | `t.Skip(...)` + `runtime.GOOS` for platform-aware tests |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go.uber.org/goleak` | v1.3.0 | Goroutine leak detection | Already in all dispatcher tests via `defer goleak.VerifyNone(t)` |

**Installation:** No new dependencies needed.

## Architecture Patterns

### Pattern 1: Thread-Safe Test Logger for zerolog
**What:** A mutex-wrapped `io.Writer` given to `zerolog.New()` prevents concurrent write races when multiple goroutines log simultaneously in tests.
**When to use:** Any test that creates a `zerolog.Logger` backed by a `bytes.Buffer` and passes it to code that logs from multiple goroutines.
**Example:**
```go
// Source: zerolog docs / Go stdlib pattern
type safeBuffer struct {
    mu  sync.Mutex
    buf bytes.Buffer
}

func (sb *safeBuffer) Write(p []byte) (n int, err error) {
    sb.mu.Lock()
    defer sb.mu.Unlock()
    return sb.buf.Write(p)
}

func (sb *safeBuffer) String() string {
    sb.mu.Lock()
    defer sb.mu.Unlock()
    return sb.buf.String()
}
```

### Pattern 2: Silent Logger for Tests That Don't Inspect Logs
**What:** `zerolog.Nop()` returns a logger that discards all output with zero allocations.
**When to use:** `newTestDispatcher()` helper — tests that only care about protocol output, not log content.
**Example:**
```go
// Source: zerolog docs
log := zerolog.Nop()
```

### Pattern 3: Platform Skip Guard
**What:** `t.Skip()` with `runtime.GOOS` check to mark tests as platform-specific.
**When to use:** Any test that exercises OS-specific path semantics.
**Example:**
```go
func TestValidatePathWindowsTraversal(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows path semantics only apply on Windows")
    }
    // ... existing test body ...
}
```

### Pattern 4: Deleting Dead Code
**What:** Remove the dead type, constructor, method, and its associated tests in a single commit. Do not leave stubs or TODOs.
**When to use:** When a type has zero production callers and its tests are the only callers.
**Example approach for `ChannelRateLimiter`:**
- Delete lines 10–52 in `ratelimit.go` (the `ChannelRateLimiter` type + `NewChannelRateLimiter` + `Allow`)
- Delete `TestRateLimiterAllow`, `TestRateLimiterPerChannel`, `TestRateLimiterConcurrent` from `ratelimit_test.go` (lines 9–114)
- Remove now-unused `"time"` import from `ratelimit_test.go` if no other test uses it

### Anti-Patterns to Avoid
- **Patching `bytes.Buffer` with `ioutil.Discard`:** Loses test coverage for `TestStructuredLogging` which must verify log field content.
- **Adding mutex to production `Dispatcher`:** The race is only in tests; production code uses a single structured logger safely. Don't change dispatcher.go.
- **Platform-sniffing in `ValidatePath` itself:** The function is correct for its platform; the test is wrong to expect Windows-only behavior on all platforms.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Thread-safe logger | Custom sync primitives | `sync.Mutex` + `io.Writer` wrapper | 5-line idiom, zero deps |
| Platform detection | Build tags or custom env check | `runtime.GOOS` in test | Standard Go pattern |
| Noop logger | `ioutil.Discard` + wrapper | `zerolog.Nop()` | Built-in, zero allocation |

## Common Pitfalls

### Pitfall 1: Stale Import After Deleting ChannelRateLimiter
**What goes wrong:** `ratelimit_test.go` imports `"time"` — used by `TestRateLimiterConcurrent` (lines 100–113 check `delay > 2*time.Minute`). After deleting those three test functions, if no other test in the file uses `time`, the import must also be removed or `go build` will fail.
**Why it happens:** Go treats unused imports as compilation errors.
**How to avoid:** After deleting the three test functions, run `go build ./internal/security/...` and fix any unused import error.
**Warning signs:** `imported and not used: "time"` compiler error.

### Pitfall 2: session package — decide "wire or remove" before touching files
**What goes wrong:** Attempting to wire `MigrateSessionHistory` into main.go while the production use-case has diverged enough that sessions are managed inline in the dispatcher (not via the session package at all).
**Why it happens:** The session package's `Worker`/`Session` design served the Telegram bot architecture (one session per chat). The dispatcher in Phase 13 replaced this with direct `claude.NewProcess` per `run` command.
**How to avoid:** Check whether `PersistenceManager` and `SessionStore` are referenced anywhere in production. They are not (confirmed: no production import of `internal/session`). The package is Telegram-era scaffolding that was partially migrated but never wired in.
**Recommendation:** Remove the entire `internal/session` package. The dispatcher manages session IDs directly via `proc.SessionID()` in `runInstance()`. The session persistence concept (saving session IDs to disk for resume) is not currently wired into the dispatcher at all — that is a future feature, not Phase 17 scope.

### Pitfall 3: TestValidatePathWindows must still pass after fixing TestValidatePathWindowsTraversal
**What goes wrong:** Over-eagerly skipping both Windows path tests on macOS.
**Why it happens:** `TestValidatePathWindows` tests `ValidatePath` accepting a Windows-style path string — this actually passes on macOS because `filepath.ToSlash` normalises the path. Only the traversal test fails because traversal resolution requires OS-level path separator knowledge.
**How to avoid:** Only add the skip guard to `TestValidatePathWindowsTraversal`, not to `TestValidatePathWindows`.

### Pitfall 4: Race in TestStructuredLogging specifically requires inspectable output
**What goes wrong:** Replacing the `bytes.Buffer` logger with `zerolog.Nop()` in `TestStructuredLogging` causes the test assertions on log output (lines 1011–1021) to always fail because there is no output.
**How to avoid:** Use `safeBuffer` (mutex-wrapped) for `TestStructuredLogging`. Use `zerolog.Nop()` for `newTestDispatcher()` helper, which is used by all other tests that don't inspect log content.

## Code Examples

### Mutex-wrapped test writer
```go
// Source: Go stdlib sync pattern — idiomatic for race-safe test buffers
type safeBuffer struct {
    mu  sync.Mutex
    buf bytes.Buffer
}

func (sb *safeBuffer) Write(p []byte) (n int, err error) {
    sb.mu.Lock()
    defer sb.mu.Unlock()
    return sb.buf.Write(p)
}

func (sb *safeBuffer) String() string {
    sb.mu.Lock()
    defer sb.mu.Unlock()
    return sb.buf.String()
}
```

### newTestDispatcher using Nop logger (no race, no log inspection)
```go
log := zerolog.Nop()
d := New(conn, cfg, nodeCfg, auditLog, limiter, log)
```

### TestStructuredLogging using safeBuffer
```go
var logBuf safeBuffer
log := zerolog.New(&logBuf)
// ... later ...
logOutput := logBuf.String()
```

### Platform guard for Windows-only traversal test
```go
func TestValidatePathWindowsTraversal(t *testing.T) {
    if runtime.GOOS != "windows" {
        t.Skip("Windows path traversal test requires Windows filepath.Clean semantics")
    }
    // existing body unchanged
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `ChannelRateLimiter` (Telegram channel ID int64) | `ProjectRateLimiter` (string project name) | Phase 13 | Channel limiter is dead code |
| `internal/session` package wired into bot handlers | Dispatcher manages instances directly | Phase 13 | Session package is dead code |
| `IsAuthorized(userID, channelID, allowedUsers)` | No user/channel auth (WebSocket token auth) | Phase 12 | `IsAuthorized` is dead code (not in scope for Phase 17 success criteria, but worth noting) |

## Open Questions

1. **Should `IsAuthorized` in `validate.go` also be removed?**
   - What we know: Not imported anywhere in production. Phase 17 success criteria only names `ChannelRateLimiter`. `IsAuthorized` is a Telegram concept (userID int64, channelID int64).
   - What's unclear: Whether a future phase might reuse the function concept with different parameters.
   - Recommendation: Remove it as part of this phase — it's Telegram dead code, no production caller exists, and its test (`TestIsAuthorizedChannelIDAccepted`) explicitly says "Phase 2 forward-compatibility" which was never implemented. Keep the removal surgical to avoid unplanned scope creep.

2. **Should `internal/session` tests be deleted along with the package?**
   - What we know: If the whole package is removed, the tests go with it (`session_test.go`, `store_test.go`, `persist_test.go`, `migrate_test.go`).
   - What's unclear: None — deleting a package deletes its tests. This is the correct approach if removing.
   - Recommendation: Remove the package in its entirety.

## Environment Availability

Step 2.6: SKIPPED (no external dependencies — this phase is code/test changes only, Go toolchain already confirmed working by `go test` output above).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib testing + go test |
| Config file | none (standard `go test` invocation) |
| Quick run command | `go test ./internal/security/... ./internal/dispatch/...` |
| Full suite command | `go test -race ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CLEAN-02 | ChannelRateLimiter deleted | unit (compile + run) | `go test ./internal/security/...` | Existing tests in ratelimit_test.go will be deleted as part of the change |
| CLEAN-04 | session package wired or removed | unit (build check) | `go test ./...` (no session import errors) | Existing session tests deleted if package removed |
| SC-1 | ChannelRateLimiter type and tests gone | structural | `go vet ./internal/security/...` | N/A — deletion |
| SC-2 | internal/session wired or removed | structural | `go build ./...` | N/A — deletion or wiring |
| SC-3 | Dispatch tests race-clean | race detector | `go test -race ./internal/dispatch/...` | ✅ dispatcher_test.go |
| SC-4 | TestValidatePathWindowsTraversal platform-aware | unit | `go test ./internal/security/...` | ✅ validate_test.go |

### Sampling Rate
- **Per task commit:** `go test ./internal/security/... ./internal/dispatch/...`
- **Per wave merge:** `go test -race ./...`
- **Phase gate:** `go test -race ./...` green before `/gsd:verify-work`

### Wave 0 Gaps
None — existing test infrastructure covers all phase requirements. No new test files needed; Phase 17 is deletion + fixes on existing files.

## Sources

### Primary (HIGH confidence)
- Direct codebase inspection — all findings verified by reading source files and running `go test -race ./...`
- Go stdlib `runtime.GOOS` docs — standard cross-platform detection
- zerolog v1.34.0 source (in go module cache) — `zerolog.Nop()` confirmed in writer.go

### Secondary (MEDIUM confidence)
- `bytes.Buffer` thread-safety: documented as NOT goroutine-safe in Go stdlib docs — concurrent writes require external synchronisation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new libraries; all findings from direct code inspection
- Architecture: HIGH — race root cause confirmed by `go test -race` output; traversal failure confirmed by `go run` reproduction
- Pitfalls: HIGH — all pitfalls discovered by reading the actual failing tests

**Research date:** 2026-03-25
**Valid until:** Stable — no external dependencies to go stale
