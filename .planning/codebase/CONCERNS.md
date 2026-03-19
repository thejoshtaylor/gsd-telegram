# Codebase Concerns

**Analysis Date:** 2026-03-19

## Tech Debt

**Silent Empty Error Handlers:**
- Issue: Numerous `catch` blocks with empty bodies or minimal logging (`catch {}`), especially in message deletion operations
- Files: `src/handlers/callback.ts` (lines 70, 79, 433), `src/handlers/document.ts` (lines 329, 359), `src/handlers/streaming.ts` (lines 132-138, 167, 192), `src/handlers/audio.ts` (line 130)
- Impact: Errors silently fail, making debugging difficult. Network errors, permission issues, and race conditions go unnoticed
- Fix approach: Replace empty catches with at least minimal logging. Consider categorizing errors (transient vs. permanent)

**Windows-Specific Path Handling Fragility:**
- Issue: Multiple platform-specific branches for Windows (`shell: process.platform === "win32"`) in `src/session.ts`, PDF extraction uses `pdftotext` CLI requiring `brew install poppler`
- Files: `src/session.ts` (lines 38, 346), `src/handlers/document.ts` (lines 162-172)
- Impact: Deployment on Windows requires manual PATH configuration. Missing pdftotext causes silent failures with unclear error messages
- Fix approach: Create platform abstraction layer. Add runtime validation for external dependencies (pdftotext, claude CLI). Provide clearer error messages when tools are missing

**Spin-Wait Loop in File Access:**
- Issue: `src/handlers/commands.ts` lines 61-64 uses busy-wait spin loop for Windows file lock resolution
- Impact: Blocks event loop, consumes CPU unnecessarily during file contention
- Fix approach: Replace with exponential backoff and proper Promise-based retry

**Session/State File Lock Vulnerability:**
- Issue: `src/handlers/commands.ts` (lines 40-68) retry loop with spin-wait for ROADMAP.md reads; `src/session.ts` (lines 670-705) session file updates can collide under concurrent requests
- Files: `src/session.ts`, `src/handlers/commands.ts`
- Impact: Rapid consecutive requests could corrupt session history or lose state updates
- Fix approach: Implement atomic file writes (write-to-temp-then-rename), use file-level locking or queued access

**Archive Extraction Size Limits Not Enforced:**
- Issue: `src/handlers/document.ts` (lines 215-254) extracts up to 50KB of content from archives but doesn't verify total decompressed size
- Impact: Bomb.zip-style attacks could cause memory exhaustion
- Fix approach: Add max decompressed size check; limit individual file extraction; validate extraction boundaries before reading

## Known Bugs

**Media Group Timeout Race Condition:**
- Symptoms: Occasionally processes incomplete photo/document albums if timeout fires before all items arrive
- Files: `src/handlers/media-group.ts` (lines 140-144, 156-160)
- Trigger: Send multiple photos rapidly (within 1s window) and network is slow
- Cause: Fixed 1s timeout (MEDIA_GROUP_TIMEOUT) doesn't account for Telegram's variable delivery timing
- Workaround: Resend the album or wait longer between items

**Ask-User Button Retry Loop Timing:**
- Symptoms: Sometimes ask_user buttons appear after response ends or don't appear at all
- Files: `src/session.ts` (lines 496-511)
- Trigger: High-load conditions or network latency
- Cause: 200ms sleep + 3 retries with 100ms waits (lines 497-510) may be insufficient; file system check is not atomic
- Impact: User misses choices, query doesn't progress

**PDF Extraction Silent Failure:**
- Symptoms: "PDF parsing failed" message appears but user doesn't know pdftotext is missing
- Files: `src/handlers/document.ts` (lines 104-116)
- Trigger: pdftotext not installed or not in PATH
- Cause: Error is caught and generic message returned
- Workaround: Ensure `poppler` is installed; add PATH validation on startup

**Empty Archive Handling:**
- Symptoms: Bot shows "0 files, 0 readable" but still sends "Processing..." to Claude
- Files: `src/handlers/document.ts` (lines 291-301)
- Cause: No validation that extracted content is non-empty before proceeding
- Impact: Wastes API tokens on empty prompts

## Security Considerations

**Insufficient rm Command Parsing:**
- Risk: Simple regex-based parsing (`src/security.ts` lines 136-148) can be bypassed with shell tricks like `rm "$(cat /etc/passwd)"` or `rm $(find / -name important)`
- Files: `src/security.ts`
- Current mitigation: Basic path allowlist check, but shell expansion happens before path validation
- Recommendations: Parse rm commands more robustly; consider blocking `rm` entirely and require explicit `/delete` command instead

**Environment Variable Leakage in System Prompt:**
- Risk: `TRANSCRIPTION_CONTEXT_FILE` path is read from env but could expose paths/patterns
- Files: `src/config.ts` (lines 149-155)
- Current mitigation: File is read at startup, not exposed to Claude
- Recommendations: Sanitize any file contents before use; document what should/shouldn't be in context files

**Session ID File Permissions:**
- Risk: Session file (`/tmp/claude-telegram-session.json`) stores session IDs as plaintext, readable by any user on the system
- Files: `src/session.ts` (lines 671-705)
- Current mitigation: Uses /tmp (typically world-readable on Unix)
- Recommendations: Create `.session.json` with `0600` permissions; encrypt session IDs at rest

**Rate Limiter Memory Leak:**
- Risk: `src/security.ts` (lines 21-76) buckets map stores entries for every user forever, no cleanup
- Impact: Long-running bot will accumulate stale user entries
- Fix approach: Add expiration tracking to buckets; clean up entries older than (e.g.) 24 hours

**Ask-User File Cleanup:**
- Risk: `src/handlers/streaming.ts` (lines 52-54) scans `/tmp` for all `ask-user-*.json` files without ownership validation
- Impact: Could process requests from other bot instances or orphaned sessions
- Fix approach: Include user ID or session ID in filename; validate ownership before processing

## Performance Bottlenecks

**Blocking PDF Extraction:**
- Problem: `src/handlers/document.ts` (line 107) uses `execSync` for pdftotext, blocking event loop during conversion
- Files: `src/handlers/document.ts`
- Cause: Large PDFs (>1MB) can block for several seconds
- Improvement path: Use async `exec` or spawn subprocess with streaming; add timeout guard

**Archive Content Accumulation:**
- Problem: `src/handlers/document.ts` (lines 221-254) reads entire file list into memory before filtering
- Files: `src/handlers/document.ts`
- Cause: Deep directory trees with many files cause memory spike
- Improvement path: Use streaming/iterator approach; stop early when limit is reached

**Streaming Throttle Latency:**
- Problem: 500ms throttle (`STREAMING_THROTTLE_MS` in `src/config.ts` line 190) means status updates lag for fast responses
- Impact: User sees stale progress indicators
- Improvement path: Make throttle adaptive based on message rate; use incremental updates

**Session Query Timeout Soft Check:**
- Problem: No hard timeout on `sendMessageStreaming()` — relies on CLI process to exit
- Files: `src/session.ts` (lines 251-654)
- Cause: Stuck Claude CLI subprocess consumes resources indefinitely
- Fix approach: Add `QUERY_TIMEOUT_MS` enforcement with process kill-on-timeout

## Fragile Areas

**Conversation Title Truncation Logic:**
- Files: Multiple handlers (e.g., `src/handlers/photo.ts` lines 80-83, `src/handlers/document.ts` lines 304-307)
- Why fragile: Truncate to 47 chars + "..." is hardcoded; doesn't account for multibyte UTF-8 characters that could break in the middle
- Safe modification: Use `text.substring(0, N)` with proper UTF-8 boundary detection
- Test coverage: No tests for non-ASCII titles

**Media Group Buffer Timeout Callback:**
- Files: `src/handlers/media-group.ts` (lines 140-144)
- Why fragile: Timeout captures `processCallback` reference; if handler is reloaded, callback may reference stale code
- Safe modification: Store callback ID and look up at timeout firing time
- Test coverage: No tests for module reloading scenarios

**Ask-User Request File Synchronization:**
- Files: `src/handlers/streaming.ts` (lines 46-85), `src/handlers/callback.ts` (lines 127-141)
- Why fragile: Two separate read/write operations on same file without locking
- Status: Race condition possible if button clicked while file being written
- Test coverage: No concurrent scenario tests

**Path Normalization Edge Cases:**
- Files: `src/security.ts` (lines 80-116)
- Why fragile: Symlink resolution with fallback (`realpathSync` → `resolve`); `sep` usage may not handle mixed separators on Windows
- Safe modification: Always use forward slashes or `path.posix`; test symlink scenarios
- Test coverage: No symlink or relative path tests

**Error Message Truncation:**
- Files: `src/session.ts` (line 612), `src/handlers/callback.ts` (line 694)
- Why fragile: `.slice(0, 100)` truncates in the middle of multi-byte sequences; error text may be misleading
- Safe modification: Use proper UTF-8-aware truncation
- Test coverage: No error message format validation

## Scaling Limits

**In-Memory Session Cache:**
- Current capacity: `MAX_SESSIONS = 5` hardcoded in `src/session.ts` line 75
- Limit: Bot supports only one active session at a time; second user blocks until first completes
- Impact: Multiple concurrent users will queue/block
- Scaling path: Implement per-user sessions with separate CLI subprocess pools; add configurable queue limits

**Message Queue Depth:**
- Current capacity: `MAX_QUEUE_SIZE = 5` in `src/session.ts` line 77
- Limit: 6th message during processing is rejected
- Impact: Rapid fire messages will be lost
- Scaling path: Increase queue size with memory awareness; add priority queue logic

**Archive Extraction Buffer:**
- Current capacity: `MAX_ARCHIVE_CONTENT = 50000` chars (line 59 of `src/handlers/document.ts`)
- Limit: Large archives silently truncate
- Impact: User doesn't know content was truncated
- Scaling path: Stream archive processing; process in chunks; warn user when truncated

**Rate Limiter Bucket Storage:**
- Current capacity: Unbounded map of user buckets (never cleaned)
- Limit: After 1 month of operation with 1000+ unique users, map contains 1000+ stale entries
- Impact: Minor memory leak; lookup time stays O(1) but memory grows
- Scaling path: Add TTL to buckets; periodically sweep stale entries

## Dependencies at Risk

**better-sqlite3 Unused:**
- Risk: `package.json` includes `better-sqlite3@^12.6.2` but no imports found in codebase
- Impact: Unnecessary production dependency; creates build/deployment bloat
- Migration plan: Verify it's not used; remove if unused. If needed for future features, document intent

**OpenAI API Dependency:**
- Risk: Voice transcription requires `OPENAI_API_KEY` but fails gracefully if missing
- Impact: Feature disabled without warning; user confusion about why voice doesn't work
- Current mitigation: `TRANSCRIPTION_AVAILABLE` gate
- Recommendations: Add startup warning if voice commands are available but disabled; document OpenAI cost implications

**External CLI Dependencies (pdftotext, claude):**
- Risk: Hard dependency on external tools not bundled; discovery at runtime
- Files: `src/config.ts` (lines 32-52)
- Impact: Deployment failures are late and unclear
- Recommendations: Bundle detection script; fail fast with clear instructions; provide prebuilt binary option

## Missing Critical Features

**No Concurrent Session Support:**
- Problem: Multiple users queued on same session; second user waits for first to finish
- Blocks: Can't handle more than 1 active request at a time
- Workaround: User can manually use `/stop` to clear session

**No Session Persistence Across Crashes:**
- Problem: Session lost if bot process dies
- Impact: Lost conversation context
- Current mitigation: `/resume` picks up saved sessions but only if bot restarts gracefully
- Gap: Unhandled promise rejection or SIGKILL clears session without saving

**No Rate Limit Reset on Restart:**
- Problem: Rate limiter buckets persist in memory but lost on restart
- Impact: Legitimate user spike after restart not rate limited properly
- Recommendation: Persist rate limit state or implement sliding window on startup

**No Audit Log Rotation:**
- Problem: Audit log grows unbounded (`src/config.ts` line 196)
- Impact: After months, log file becomes multi-GB
- Recommendation: Implement log rotation (e.g., daily, max 10 files)

## Test Coverage Gaps

**No Unit Tests:**
- What's not tested: Session message streaming, rate limiter token bucket math, path validation edge cases
- Files: `src/session.ts`, `src/security.ts`
- Risk: Refactoring session logic could break streaming; rate limit calculations may drift
- Priority: High — session.ts is the most complex module

**No Integration Tests:**
- What's not tested: Media group buffering with actual photo/document sequences, ask_user request lifecycle
- Files: `src/handlers/media-group.ts`, `src/handlers/streaming.ts`
- Risk: Timeout race conditions go unnoticed; ask_user button flow fragile
- Priority: High — these are user-facing flows

**No Error Scenario Tests:**
- What's not tested: PDF parsing failures, archive extraction bombs, network errors during file download
- Files: `src/handlers/document.ts`, `src/handlers/photo.ts`
- Risk: Silent failures, unhelpful error messages to user
- Priority: Medium — affects user experience but not critical

**No Security Tests:**
- What's not tested: Path traversal attacks, shell injection in rm commands, symlink escape attempts
- Files: `src/security.ts`
- Risk: Security checks silently fail; malicious input could escape sandbox
- Priority: Critical — security regression could expose system files

---

*Concerns audit: 2026-03-19*
