# Codebase Structure

**Analysis Date:** 2026-03-19

## Directory Layout

```
gsd-tele/
├── src/                          # TypeScript source code
│   ├── index.ts                  # Main entry point, bot initialization
│   ├── types.ts                  # Shared TypeScript interfaces
│   ├── config.ts                 # Environment parsing, constants, safety rules
│   ├── session.ts                # ClaudeSession class (CLI subprocess management)
│   ├── security.ts               # RateLimiter, path validation, command safety
│   ├── utils.ts                  # Audit logging, voice transcription, typing indicator
│   ├── formatting.ts             # Markdown→HTML conversion, tool status formatting
│   ├── registry.ts               # Project registry parsing (markdown table format)
│   ├── vault-search.ts           # Vault search CLI wrapper
│   ├── autodoc.ts                # Automatic documentation generation
│   └── handlers/                 # Message and command handlers
│       ├── index.ts              # Re-exports all handlers
│       ├── commands.ts           # /start, /new, /clear, /stop, /status, /resume, /restart, /retry, /search, /project, /gsd
│       ├── text.ts               # Text message routing and processing
│       ├── voice.ts              # Voice→OpenAI Whisper→text
│       ├── audio.ts              # Audio file→Whisper→text
│       ├── photo.ts              # Image analysis (Claude vision)
│       ├── video.ts              # Video analysis
│       ├── document.ts           # PDF extraction, text files, archives
│       ├── callback.ts           # Inline keyboard button callbacks (resume, project, gsd, action buttons)
│       ├── streaming.ts          # StreamingState, StatusCallback factory, ask_user MCP integration
│       └── media-group.ts        # Album buffering (1s timeout for collections of media)
├── ask_user_mcp/                 # Ask-user MCP server implementation
│   └── server.ts                 # MCP tool server (spawned as subprocess)
├── mcp-config.example.ts         # Example MCP server configuration
├── mcp-config.ts                 # MCP server definitions (git-ignored)
├── package.json                  # Dependencies, scripts
├── tsconfig.json                 # TypeScript configuration
├── .env.example                  # Environment variable template
├── .env                          # Environment config (git-ignored)
├── launchagent/                  # macOS service definitions
└── README.md                     # Documentation
```

## Directory Purposes

**`src/`:**
- Purpose: All application TypeScript source code
- Contains: Modules for bot core, handlers, session management, security, utilities
- Key files: `index.ts` (entry point), `session.ts` (CLI subprocess), `handlers/` (message routing)

**`src/handlers/`:**
- Purpose: Request handlers for each message type and command
- Contains: 11 handler files + streaming state factory + media group buffer
- Key files: `commands.ts` (all commands), `text.ts` (text routing), `streaming.ts` (response accumulation)

**`ask_user_mcp/`:**
- Purpose: Model Context Protocol server for ask_user tool
- Contains: Standalone MCP server process spawned by Claude CLI
- Usage: Allows Claude to present multiple-choice buttons via Telegram (instead of text prompts)

**`launchagent/`:**
- Purpose: macOS service launcher template
- Contains: `.plist` template for running bot as background service via launchd

## Key File Locations

**Entry Points:**
- `src/index.ts`: Bot creation, handler registration, polling loop (166 lines)
- `ask_user_mcp/server.ts`: MCP tool server (when Claude invokes ask_user tool)

**Configuration:**
- `src/config.ts`: All env vars, file paths, safety constraints (244 lines)
- `.env`: Environment variables (git-ignored)
- `mcp-config.ts`: MCP server definitions (git-ignored)

**Core Logic:**
- `src/session.ts`: ClaudeSession class managing CLI subprocess (788 lines)
- `src/handlers/streaming.ts`: StatusCallback factory for streaming responses (234 lines)

**Security & Validation:**
- `src/security.ts`: RateLimiter, path validation, command safety (168 lines)
- `src/handlers/callback.ts`: Button click routing (requires auth check) (18K lines)

**Message Handlers:**
- `src/handlers/commands.ts`: All slash commands (20K lines)
- `src/handlers/text.ts`: Text message routing and special handling (8K lines)
- `src/handlers/document.ts`: PDF/text/archive extraction (18K lines)
- `src/handlers/voice.ts`: Voice→Whisper API (5.5K lines)

**Formatting & Utils:**
- `src/formatting.ts`: Markdown→HTML, tool status display (13.8K lines)
- `src/utils.ts`: Audit logging, transcription, typing indicator (5.5K lines)

**Domain Logic:**
- `src/registry.ts`: Project registry parsing (5.6K lines)
- `src/vault-search.ts`: Vault search wrapper (5.1K lines)
- `src/autodoc.ts`: Auto-documentation generation (14.6K lines)

**Types:**
- `src/types.ts`: Shared TypeScript interfaces (StatusCallback, RateLimitBucket, SavedSession, etc.)

## Naming Conventions

**Files:**
- **Handler files:** Lowercase, match message type: `commands.ts`, `text.ts`, `voice.ts`, `photo.ts`, `document.ts`, `callback.ts`
- **Core modules:** Lowercase, function: `session.ts`, `config.ts`, `security.ts`, `utils.ts`, `formatting.ts`
- **Domain files:** Descriptive lowercase: `registry.ts`, `vault-search.ts`, `autodoc.ts`

**Directories:**
- **Feature-grouped:** `handlers/` groups all message/command handlers
- **Root domain:** MCP tool in `ask_user_mcp/`
- **Config:** Root level for `mcp-config.ts`, `.env`, `package.json`

**Functions:**
- **Handler functions:** `handle[Type]` pattern: `handleText()`, `handleVoice()`, `handleDocument()`, `handleCallback()`
- **Command functions:** `handle[Command]` pattern: `handleStart()`, `handleNew()`, `handleStop()`, `handleResume()`
- **Validation functions:** `is[Property]` or `check[Property]`: `isAuthorized()`, `isPathAllowed()`, `checkCommandSafety()`
- **Factory/Creator functions:** `create[Thing]` or `[thing]Factory()`: `createStatusCallback()`, `createMediaGroupBuffer()`
- **Callback functions:** `on[Event]` or `[action]Callback()`: `statusCallback()`, `createStatusCallback()`

**Classes:**
- **PascalCase:** `ClaudeSession`, `RateLimiter`, `StreamingState`

**Variables:**
- **camelCase:** `userId`, `chatId`, `sessionId`, `rateLimiter`, `streamingState`
- **UPPER_SNAKE_CASE for constants:** `TELEGRAM_TOKEN`, `ALLOWED_USERS`, `QUERY_TIMEOUT_MS`
- **Private members:** `_workingDir`, `_isProcessing` (TypeScript convention)

## Where to Add New Code

**New Command:**
1. Add handler function in `src/handlers/commands.ts`: `export async function handleMyCommand(ctx: Context) { ... }`
2. Register in `src/index.ts`: `bot.command("mycommand", handleMyCommand);`
3. Export from `src/handlers/index.ts`
4. Add to command menu in `src/index.ts` bot startup

**New Message Handler (New Type):**
1. Create file: `src/handlers/[type].ts`
2. Export `async function handle[Type](ctx: Context) { ... }`
3. Import and export from `src/handlers/index.ts`
4. Register in `src/index.ts`: `bot.on("message:[type]", handle[Type]);`

**New Utility Function:**
1. If cross-cutting (logging, validation): Add to `src/utils.ts` or `src/security.ts`
2. If formatting-related: Add to `src/formatting.ts`
3. If domain-specific (project registry, vault): Create in `src/[domain].ts`

**New Handler Middleware:**
1. Add middleware logic in `src/index.ts` after auto-retry, before handler registration
2. Use grammY middleware pattern: `bot.use((ctx, next) => { ... await next(); })`

**Configuration Addition:**
1. Add to `src/config.ts` in appropriate section (Core, Security, Media Group, etc.)
2. Read from env var with fallback default
3. Export constant for use across codebase
4. Document in `.env.example`

## Special Directories

**`/tmp/` (Runtime Temp Files):**
- Purpose: Session persistence, restart coordination, ask_user MCP communication
- Generated: Yes (at runtime)
- Committed: No (local files)
- Files:
  - `claude-telegram-session.json`: Multi-session history (max 5 per working dir)
  - `claude-telegram-state.json`: Current working directory (survives restarts)
  - `claude-telegram-restart.json`: Pending restart message (updated on bot restart)
  - `claude-telegram-audit.log`: Audit log of all user actions
  - `ask-user-{requestId}.json`: Temporary ask_user MCP request/response files
  - `telegram-bot/`: Downloaded media (photos, documents, audio files)

**`node_modules/`:**
- Purpose: Installed dependencies
- Generated: Yes (via `npm install`)
- Committed: No (git-ignored)

**`launchagent/`:**
- Purpose: macOS service launcher templates
- Generated: No (checked in)
- Committed: Yes
- Usage: Copy `.plist.template` to `~/Library/LaunchAgents/`, edit paths, run `launchctl load`

## Message Flow By Type

**Text Message:**
```
telegram update
  → bot.on("message:text")
  → handleText(ctx)
    → isAuthorized()
    → rateLimiter.check()
    → checkInterrupt() (! prefix handling)
    → session.sendMessageStreaming()
    → createStatusCallback() accumulates response
    → converts markdown to HTML
    → splits by 4096 char limit
    → sends Telegram messages
```

**Command:**
```
telegram update
  → bot.command("name")
  → handleCommand(ctx)
    → isAuthorized()
    → execute command logic
    → may call session.sendMessageStreaming() for longer ops
    → send response
```

**Media (photo/document/voice/audio):**
```
telegram update
  → bot.on("message:[type]")
  → handle[Type](ctx)
    → isAuthorized()
    → rateLimiter.check()
    → download from Telegram
    → process (transcribe, extract, analyze)
    → feed to text handler or send to Claude
```

**Button Click:**
```
telegram callback query
  → bot.on("callback_query:data")
  → handleCallback(ctx)
    → isAuthorized()
    → parse callback data (e.g., "resume:session_id")
    → route to specific handler
    → may trigger session resume or command
```

## Build & Run

**Development:**
- `npm install` - Install dependencies
- `npm run dev` - Run with auto-reload (--watch)
- `npm run typecheck` - Run TypeScript type checking
- `npm run test` - Run vitest unit tests

**Production:**
- `npm run start` - Start bot (single run, no watch)
- `npm run build` (if added) - Compile to binary via `bun build --compile`

**Environment Setup:**
- Copy `.env.example` to `.env`
- Fill in: `TELEGRAM_BOT_TOKEN`, `TELEGRAM_ALLOWED_USERS`, optional vars
- Ensure `claude` CLI is in PATH (or set `CLAUDE_CLI_PATH`)

---

*Structure analysis: 2026-03-19*
