# Architecture

**Analysis Date:** 2026-03-19

## Pattern Overview

**Overall:** Event-driven request-response pipeline with long-lived Claude CLI subprocess management

**Key Characteristics:**
- Stateful CLI session management (Claude AI subprocess maintained across multiple turns)
- Message queuing and sequentialization per user (prevents race conditions)
- Streaming NDJSON parsing for incremental Claude response delivery
- Multi-layer security (allowlist, rate limiting, path validation, command safety checks)
- Defense-in-depth safety constraints via system prompts and command filtering
- Multi-media input support (text, voice, audio, video, photos, documents, PDFs)

## Layers

**Telegram Bot Layer:**
- Purpose: Handle incoming Telegram updates and dispatch to appropriate handlers
- Location: `src/index.ts`
- Contains: Bot initialization, middleware registration (sequentialization, auto-retry), command registration, error handling
- Depends on: grammY framework, handlers (`src/handlers/`)
- Used by: Entry point for all message/command flows

**Request Handler Layer:**
- Purpose: Route and process different message types with auth and rate limiting
- Location: `src/handlers/*.ts`
- Contains: 11 handler files (commands, text, voice, audio, photo, document, video, callback, streaming, media-group)
- Depends on: Session, Security, Config, Utils, Formatting modules
- Used by: Bot layer registers handlers

**Session Management Layer:**
- Purpose: Manage Claude CLI subprocess lifecycle, streaming, session persistence
- Location: `src/session.ts`
- Contains: `ClaudeSession` class wrapping Claude CLI as subprocess, NDJSON event parsing, session history persistence, state recovery
- Depends on: Config, Formatting, Types
- Used by: All handlers for sending messages to Claude

**Security & Validation Layer:**
- Purpose: Enforce access control, rate limiting, path validation, command safety
- Location: `src/security.ts`
- Contains: `RateLimiter` class (token bucket algorithm), path allowlist enforcement, command pattern blocking
- Depends on: Config, Types
- Used by: All request handlers before processing

**Configuration & Constants Layer:**
- Purpose: Centralize all environment variables, file paths, safety constraints
- Location: `src/config.ts`
- Contains: Environment parsing, MCP server loading, safety prompts, rate limit settings, file paths
- Depends on: Environment variables, `mcp-config.ts`
- Used by: All layers for configuration lookup

**Utility & Formatting Layer:**
- Purpose: Cross-cutting concerns (audit logging, voice transcription, markdown conversion, tool status formatting)
- Location: `src/utils.ts`, `src/formatting.ts`
- Contains: OpenAI transcription, audit logging, HTML/markdown conversion, tool status display formatting
- Depends on: Config, Types, OpenAI client
- Used by: Handlers, Session, Formatting

**Domain-Specific Modules:**
- Location: `src/registry.ts`, `src/vault-search.ts`, `src/autodoc.ts`
- Contains: Project registry parsing (markdown table), vault search CLI, autodoc generation
- Depends on: Config, Utils
- Used by: Command handlers for project switching, documentation generation

## Data Flow

**Standard Text Message Flow:**

1. **Telegram Update** → grammY deserializes and dispatches to `handleText`
2. **Authorization** → `isAuthorized()` checks user ID against `ALLOWED_USERS`
3. **Rate Limit Check** → `rateLimiter.check(userId)` validates token bucket
4. **Message Processing** →
   - Check for interrupt (`!` prefix) via `checkInterrupt()`
   - If session is running, stop it and clear message
   - Otherwise, strip prefix if present
5. **Typing Indicator** → `startTypingIndicator()` sends periodic "typing" actions
6. **Query Dispatch** → `session.sendMessageStreaming()` spawns Claude CLI subprocess
7. **NDJSON Parsing** → Each line from stdout parsed as JSON event
8. **Event Handling** → Process `assistant`, `result`, `thinking`, `tool_use` blocks
9. **Streaming Callback** → `createStatusCallback()` accumulates text and sends throttled Telegram updates
10. **Response Rendering** → Convert markdown to HTML, split by Telegram 4096 char limit, send messages
11. **Audit Logging** → `auditLog()` records user, message type, content, response

**Voice Message Flow:**

1. → `handleVoice` downloads audio file from Telegram
2. → `transcribeVoice()` sends to OpenAI Whisper API
3. → Transcription result fed to text message handler (same as step 1)

**Document/PDF Flow:**

1. → `handleDocument` downloads file, detects type (PDF, text, archive, audio)
2. → PDF: `pdftotext` CLI extracts text
3. → Archive: Extract and filter text files, concatenate content (max 50K chars)
4. → Media group buffering (1s timeout to collect multiple documents)
5. → Combined text sent to text handler

**Callback Query Flow (Button Presses):**

1. → `handleCallback` parses callback data (e.g., `resume:{session_id}`)
2. → Route to specific handler: resume, project switch, GSD operation, action button
3. → Resume: `session.resumeSession(sessionId)` restores session from disk
4. → Execute appropriate command or GSD operation

**State Management:**

- **Session State:** In-memory `ClaudeSession` object holds session_id, current tool, last activity, context %, token usage
- **Session Persistence:** Saved to `/tmp/claude-telegram-session.json` as multi-session history (max 5 sessions per working dir)
- **Working Directory State:** Saved to `/tmp/claude-telegram-state.json` so `setWorkingDir()` survives restarts
- **Rate Limit State:** Per-user token bucket tracked in memory (not persisted)
- **Ask-User MCP State:** Temporary JSON files in `/tmp/` written by ask_user MCP tool, consumed by `handleCallback`

## Key Abstractions

**ClaudeSession Class:**
- Purpose: Encapsulate Claude CLI subprocess lifecycle and streaming protocol
- Examples: `src/session.ts` (784 lines)
- Pattern: State machine with `isActive`, `isRunning`, `stopRequested` flags; NDJSON event parser with partial message deduplication
- Interface: `sendMessageStreaming()` (main API), `kill()`, `resumeSession()`, `stop()`, `queueMessage()`

**StreamingState (Callback Factory):**
- Purpose: Stateful accumulator for streaming response segments and status updates
- Examples: `src/handlers/streaming.ts` (234 lines)
- Pattern: Creates closure around message building, throttling, and Telegram message splitting
- Interface: `createStatusCallback()` returns async function triggered on "thinking", "tool", "text", "segment_end", "done" events

**RateLimiter Class:**
- Purpose: Token bucket algorithm for per-user rate limiting
- Examples: `src/security.ts` (76 lines)
- Pattern: Refill tokens based on elapsed time; check bucket before allowing message
- Interface: `check(userId)` returns `[allowed, retryAfter?]`

**MediaGroupBuffer:**
- Purpose: Collect multiple photo/document messages into single batch (1s timeout)
- Examples: `src/handlers/media-group.ts`
- Pattern: Timers with auto-expiry; prevents "album" of 5 photos sending 5 separate requests
- Interface: `createMediaGroupBuffer()` factory, `addItem()`, `flush()`

**Handler Pattern:**
- Purpose: Async function matching grammY handler signature
- Pattern: `async (ctx: Context) => void` with auth check → rate limit check → operation → response
- All handlers follow: Check user ID, authorize, rate limit, start typing indicator, dispatch work, catch/log errors

## Entry Points

**`src/index.ts` (Main Process Entry):**
- Location: `src/index.ts` (166 lines)
- Triggers: `npm run start` or `npm run dev` with auto-reload
- Responsibilities:
  - Create grammY `Bot` instance with Telegram token
  - Register auto-retry middleware for API rate limits
  - Register sequentialization middleware (queues non-command messages per user)
  - Register all command handlers (`/start`, `/new`, `/stop`, `/status`, `/resume`, `/project`, `/gsd`, etc.)
  - Register message type handlers (text, voice, photo, document, audio, video)
  - Register callback query handler for buttons
  - Detect pending restart and update message if applicable
  - Start polling with concurrent runner (commands work immediately, other messages sequentialized)

**Handler Dispatch Points:**
- `bot.command()` → Routes to `src/handlers/commands.ts` handlers
- `bot.on("message:text")` → Routes to `src/handlers/text.ts`
- `bot.on("message:voice")` → Routes to `src/handlers/voice.ts`
- `bot.on("message:document")` → Routes to `src/handlers/document.ts`
- `bot.on("callback_query:data")` → Routes to `src/handlers/callback.ts`

## Error Handling

**Strategy:** Try-catch with user feedback, audit logging, graceful degradation

**Patterns:**

1. **Authorization Failures:** Reply "Unauthorized" without processing request
2. **Rate Limit Hit:** Reply with retry-after time, audit log the limit hit
3. **Claude CLI Errors:** Capture stderr, check for "prompt too long" pattern, auto-clear session if detected
4. **File Operations:** Catch file-not-found, permission denied, provide fallback or user-friendly error message
5. **Streaming Errors:** Suppress post-stop errors (user-initiated cancel), log other errors
6. **Transcription Failures:** Return null, fall back to requesting text input
7. **Global Error Handler:** `bot.catch()` logs error to console

**Context Limit Detection:**
- Monitor stderr for patterns: "prompt too long", "exceed context limit", "conversation is too long"
- Auto-clear session on detection, return "⚠️ Context limit reached — session auto-cleared"

## Cross-Cutting Concerns

**Logging:**
- Console logs for debugging (CLI subprocess events, session lifecycle, tool calls)
- Audit log to `/tmp/claude-telegram-audit.log` for compliance (user actions, tools used, errors)
- Format: Plain text (default) or JSON (configurable via `AUDIT_LOG_JSON`)

**Validation:**
- User ID allowlist (`TELEGRAM_ALLOWED_USERS`) checked first in all handlers
- File paths validated against `ALLOWED_PATHS` before Claude processes them
- Commands checked against `BLOCKED_PATTERNS` (rm -rf /, mkfs., dd if=, etc.)
- System prompt injected with safety rules about deletions and path constraints

**Authentication:**
- Single-layer: Telegram user ID allowlist
- No token-based auth (Telegram bot token authenticates the channel)

**Session Lifecycle:**
- Sessions are created on first message in new chat/bot restart
- Sessions are resumed via `/resume` command or button callbacks
- Sessions are killed on `/new` or `/clear` commands, or auto-cleared on context limit
- At most 5 sessions retained per working directory

**Ask-User MCP Integration:**
- Claude CLI spawned with `--append-system-prompt` instructing it to use ask_user MCP for multiple-choice
- ask_user MCP tool writes JSON files to `/tmp/ask-user-{requestId}.json`
- Streaming loop detects `mcp__ask-user` tool invocation, waits for button file, sends inline keyboard
- User presses button → callback data routed to handler, JSON file read, response sent back to Claude via ask_user result

---

*Architecture analysis: 2026-03-19*
