# External Integrations

**Analysis Date:** 2026-03-19

## APIs & External Services

**Telegram Bot API:**
- Service: Telegram
- What it's used for: Command routing, message delivery, media handling, inline keyboards, bot polling
- SDK/Client: grammY 1.38.4 wrapper over Telegram Bot API
- Auth: `TELEGRAM_BOT_TOKEN` environment variable
- Endpoints:
  - `https://api.telegram.org/bot{TOKEN}/` - Main API
  - `https://api.telegram.org/file/bot{TOKEN}/{file_path}` - File downloads (voice, photos, documents)
- Rate limiting: Built-in with `@grammyjs/auto-retry` middleware

**OpenAI Whisper API:**
- Service: OpenAI (speech-to-text transcription)
- What it's used for: Converting voice messages and audio files to text
- SDK/Client: openai 6.15.0 (official Node.js SDK)
- Auth: `OPENAI_API_KEY` environment variable
- Model: `gpt-4o-transcribe` (via `openai.audio.transcriptions.create()`)
- Implementation: `src/utils.ts` - `transcribeVoice()` function
- Handlers: `src/handlers/voice.ts`, `src/handlers/audio.ts`
- Availability flag: `TRANSCRIPTION_AVAILABLE` (true only if API key configured)

**Claude AI (Local CLI):**
- Service: Anthropic Claude (via local `claude` CLI)
- What it's used for: Core conversational AI, code analysis, tool use (MCP), streaming responses
- SDK/Client: Native subprocess via Node.js `child_process.spawn()`
- Invocation: `claude -p --output-format stream-json`
- Auth: Claude MAX subscription (user's account via CLI)
- Implementation: `src/session.ts` - `ClaudeSession` class
- No API costs: Runs on user's Claude CLI subscription
- Streaming: JSON-line format parsing via `readline` interface
- MCP integration: Optional servers defined in `src/mcp-config.ts`

## Data Storage

**Databases:**
- Basic Memory Vault (external)
  - Type: SQLite with FTS5 full-text search
  - Location: `~/.basic-memory/memory.db` (configurable via `BASIC_MEMORY_DB` env var)
  - Connection: better-sqlite3 12.6.2 (readonly mode)
  - Purpose: Vault search via `/search` command
  - Implementation: `src/vault-search.ts`
  - Read-only: Never modifies vault database
  - Tables: `search_index`, `entity`

**Session Persistence (Local):**
- Type: JSON file storage
- Location: `/tmp/claude-telegram-session.json` (configurable via env)
- Purpose: Save/restore Claude conversation sessions
- Format: Array of `SavedSession` objects with `session_id`, `saved_at`, `working_dir`, `title`

**State File (Local):**
- Type: JSON file storage
- Location: `/tmp/claude-telegram-state.json`
- Purpose: Persist chat state across restarts

**File Storage:**
- Local filesystem only (no cloud storage)
- Download locations for media:
  - Photos/documents: `/tmp/telegram-bot/` (cleanup after processing)
  - Voice/audio files: `/tmp/telegram-bot/voice_*.ogg` (temporary)
  - PDF extraction: Uses `pdftotext` CLI (not npm package)
- Audit log: `/tmp/claude-telegram-audit.log` (append-only, configurable location)

**Caching:**
- Memory-based: Session state held in `ClaudeSession` class
- No distributed cache (single-process bot)
- Media group buffering: 1000ms timeout for grouping album photos

## Authentication & Identity

**Auth Provider:**
- Custom implementation with allowlist
- Auth method: Telegram user ID allowlist
- Implementation: `src/security.ts` - `isAuthorized()` function
- Configuration: `TELEGRAM_ALLOWED_USERS` env var (comma-separated user IDs)
- Scope: All non-authorized users are rejected with "Unauthorized" message
- No OAuth/third-party auth

**Rate Limiting:**
- Token bucket algorithm (not third-party service)
- Implementation: `src/security.ts` - `RateLimiter` class
- Configuration:
  - `RATE_LIMIT_ENABLED` (default: true)
  - `RATE_LIMIT_REQUESTS` (default: 20 requests)
  - `RATE_LIMIT_WINDOW` (default: 60 seconds)
- Per-user tracking with audit logging

## Monitoring & Observability

**Error Tracking:**
- None detected (no Sentry, Rollbar, etc.)
- Errors logged to console and audit log only

**Logs:**
- Console output: Bot startup, errors, Claude CLI output
- Audit log: File-based append-only log at `AUDIT_LOG_PATH`
- Format: JSON (if `AUDIT_LOG_JSON=true`) or human-readable text
- Events: Message operations, rate limit hits, auth failures, tool use, errors
- Implementation: `src/utils.ts` - `auditLog()`, `auditLogRateLimit()`

**Typing Indicators:**
- Native Telegram typing indicator (indicates bot is processing)
- Implementation: `src/utils.ts` - `startTypingIndicator()` function

## CI/CD & Deployment

**Hosting:**
- Self-hosted (user's machine or server)
- No cloud platform dependency (runs standalone Node.js or compiled binary)

**CI Pipeline:**
- None detected (no GitHub Actions, GitLab CI, etc.)

**Deployment Options:**
1. Direct Node.js execution: `npm run start`
2. With auto-reload: `npm run dev`
3. Standalone binary: `bun build --compile` → single executable
4. macOS service: launchd plist at `~/Library/LaunchAgents/` (see `launchagent/`)
5. PM2 process manager: `ecosystem.config.cjs` configuration
6. Docker: Not provided (but possible)

## Environment Configuration

**Required env vars:**
- `TELEGRAM_BOT_TOKEN` - Bot API token
- `TELEGRAM_ALLOWED_USERS` - User IDs (comma-separated)

**Optional env vars:**
- `CLAUDE_WORKING_DIR` - Where Claude executes commands (default: home dir)
- `ALLOWED_PATHS` - Paths Claude can access (default: home, Documents, Downloads, Desktop, .claude)
- `OPENAI_API_KEY` - For voice transcription (enables voice/audio handlers)
- `CLAUDE_CLI_PATH` - Path to `claude` binary (auto-detected if not set)
- `BASIC_MEMORY_DB` - Path to vault database (default: `~/.basic-memory/memory.db`)
- `AUDIT_LOG_PATH` - Location for audit log (default: `/tmp/claude-telegram-audit.log`)
- `AUDIT_LOG_JSON` - JSON format for audit logs (default: false)
- `RATE_LIMIT_ENABLED` - Enable rate limiting (default: true)
- `RATE_LIMIT_REQUESTS` - Requests per window (default: 20)
- `RATE_LIMIT_WINDOW` - Time window in seconds (default: 60)
- `TRANSCRIPTION_CONTEXT_FILE` - Optional context file for voice transcription
- `THINKING_KEYWORDS` - Trigger words for Claude thinking mode (default: "think,reason,analyze")
- `THINKING_DEEP_KEYWORDS` - Trigger words for deep thinking (default: "ultrathink,think hard,think deeply")

**Secrets location:**
- `.env` file (git-ignored via `.gitignore`)
- Create from `.env.example` template
- NEVER commit `.env` with real tokens

## Webhooks & Callbacks

**Incoming:**
- Telegram Bot API (polling-based, not webhooks)
- Message event routing via grammY handler system
- Callback query handling: Inline keyboard button presses

**Outgoing:**
- Telegram API calls (sendMessage, editMessage, setMyCommands, etc.)
- No external webhooks configured
- MCP tool callbacks: Via Claude's tool_use integration (ask-user MCP for interactive buttons)

## External Tool Integration (MCP)

**MCP Servers (optional, defined in `src/mcp-config.ts`):**

1. **ask-user** - Interactive button prompts
   - Type: stdio
   - Command: `bun run {REPO_ROOT}/ask_user_mcp/server.ts`
   - Purpose: Present Claude's questions as Telegram inline keyboard buttons
   - Callback handling: `src/handlers/callback.ts` parses `askuser:{requestId}:{optionIndex}`
   - Request files: `/tmp/ask-user-{requestId}.json`

2. **ruflo** - Example (optional)
   - Type: stdio
   - Command: `npx -y ruflo@latest mcp start`
   - Purpose: File search and navigation

3. **typefully** - Example (commented out)
   - Type: HTTP
   - URL: `https://mcp.typefully.com/mcp`
   - Purpose: Draft and schedule social media posts

4. **things** - Example (commented out, macOS only)
   - Type: stdio
   - Purpose: Task manager integration

**MCP Configuration:**
- Loaded from `src/mcp-config.ts` at startup
- Each server gets stdio or HTTP access to Claude
- Tools are available in Claude conversations automatically

## Document Handler Dependencies

**PDF Extraction:**
- Tool: `pdftotext` CLI (from Poppler package)
- Installation: `brew install poppler` (macOS)
- Invocation: `src/handlers/document.ts` via system shell
- Windows compatibility: PATH must include pdftotext binary

**Media Processing:**
- Photo handler: Downloads JPEG/PNG from Telegram, sends to Claude
- Video handler: Downloads and displays metadata, no transcription
- Audio handler: Supports mp3, m4a, ogg, wav, aac, flac, opus, wma

---

*Integration audit: 2026-03-19*
