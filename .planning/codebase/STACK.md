# Technology Stack

**Analysis Date:** 2026-03-19

## Languages

**Primary:**
- TypeScript 5.7.0 - All source code in `src/`
- JavaScript (CommonJS) - Webpack/build compatibility, native modules

**Secondary:**
- Shell/Bash - Service launch scripts, process management
- JSON - Configuration and data persistence

## Runtime

**Environment:**
- Node.js (compatible, version not pinned but requires modern LTS)

**Package Manager:**
- npm (package-lock.json 650 KB)
- Lockfile: Present (package-lock.json)

## Frameworks

**Core:**
- grammY 1.38.4 - Telegram Bot API wrapper
- @grammyjs/auto-retry 2.0.2 - Automatic retry middleware for rate limiting
- @grammyjs/runner 2.0.3 - Concurrent message handling

**Testing:**
- Vitest 4.0.18 - Test runner
- Chai (via @vitest/expect) - Assertions

**Build/Dev:**
- tsx 4.19.0 - TypeScript execution (dev/run mode)
- TypeScript 5.7.0 - Compilation and type checking

## Key Dependencies

**Critical:**
- grammy - Telegram bot framework (event routing, API calls, middleware)
- openai 6.15.0 - OpenAI API client for speech-to-text transcription
- better-sqlite3 12.6.2 - Native SQLite driver for Vault search (FTS5) via CommonJS require
- zod 4.2.1 - Runtime schema validation (config parsing)
- dotenv 16.4.0 - Environment variable loading

**Infrastructure:**
- child_process (Node.js built-in) - Claude CLI subprocess spawning with streaming JSON parsing
- fs/fs-promises (Node.js built-in) - File system operations, session persistence
- readline (Node.js built-in) - Streaming response parsing from Claude CLI
- os (Node.js built-in) - Path normalization, home directory detection
- path (Node.js built-in) - Cross-platform path handling (Windows/Unix)

**Type Definitions:**
- @types/node 22.0.0 - Node.js type definitions
- @types/better-sqlite3 7.6.13 - better-sqlite3 type definitions
- @grammyjs/types - Telegram types from grammY

## Configuration

**Environment:**
- `.env` file (must be created from `.env.example`)
- Case-sensitive environment variables (Windows-compatible path handling)
- Required variables:
  - `TELEGRAM_BOT_TOKEN` - Telegram Bot API token
  - `TELEGRAM_ALLOWED_USERS` - Comma-separated user IDs authorized to use bot
  - Optional: `CLAUDE_WORKING_DIR`, `ALLOWED_PATHS`, `OPENAI_API_KEY`, `BASIC_MEMORY_DB`, `AUDIT_LOG_PATH`

**Build:**
- `tsconfig.json` - ESNext target, bundler module resolution, strict mode enabled
- No build step needed (uses tsx for direct execution)
- Standalone binary compilation: `bun build --compile` (creates single executable)

## Platform Requirements

**Development:**
- Node.js (modern LTS, e.g., 20+)
- npm or equivalent
- Claude CLI installed and in PATH (or `CLAUDE_CLI_PATH` env var)
- OpenAI API key (optional, for voice transcription)

**Production:**
- Deployment target: macOS (standalone service via launchd), Linux (systemd), Windows (npm scripts)
- External dependencies:
  - `pdftotext` CLI (from Poppler package) - PDF extraction for document handler
  - `claude` CLI command - Core Claude interaction
  - OpenAI API (if voice transcription enabled)
  - Telegram API (https://api.telegram.org)
- Optional: SQLite database at `~/.basic-memory/memory.db` for Vault search

## Entry Points

**Application:**
- `src/index.ts` - Main bot startup, command registration, event loop with grammY runner

**Subprocess:**
- `claude -p --output-format stream-json` - Claude CLI invoked by `ClaudeSession` for conversational AI

## Runtime File Locations

- Session persistence: `/tmp/claude-telegram-session.json` (or Windows equivalent)
- State file: `/tmp/claude-telegram-state.json`
- Restart marker: `/tmp/claude-telegram-restart.json`
- Temp directory: `/tmp/telegram-bot/` (photo/document downloads, voice files)
- Audit log: `/tmp/claude-telegram-audit.log` (configurable via `AUDIT_LOG_PATH`)
- MCP servers config: `src/mcp-config.ts` (optional, defines additional Claude tools)

## Development Commands

```bash
npm install              # Install dependencies
npm run start            # Run bot with tsx
npm run dev             # Run with auto-reload (--watch)
npm run typecheck       # TypeScript type checking
npm test                # Run tests with Vitest
npm test:watch          # Watch mode for tests
```

## Compilation

Standalone binary (no Node.js required at runtime):
```bash
bun build --compile src/index.ts --outfile claude-telegram-bot
```

This creates a single executable with TypeScript compiled to native code.

---

*Stack analysis: 2026-03-19*
