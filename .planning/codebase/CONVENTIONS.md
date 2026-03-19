# Coding Conventions

**Analysis Date:** 2026-03-19

## Naming Patterns

**Files:**
- Modules use lowercase with hyphens: `rate-limiter.ts`, `media-group.ts`, `auto-retry.ts`
- Handler files in `src/handlers/` follow `[message-type].ts`: `text.ts`, `voice.ts`, `audio.ts`, `callback.ts`
- Type definitions: `types.ts` (central), config: `config.ts`, utilities: `utils.ts`
- Tests mirror source structure: `tests/[module].test.ts`

**Functions:**
- camelCase for all functions: `handleText()`, `parseRegistry()`, `isPathAllowed()`, `checkCommandSafety()`
- PascalCase for classes: `ClaudeSession`, `RateLimiter`, `StreamingState`, `InlineKeyboard`
- Private methods use leading underscore: `_workingDir`, `_isProcessing`, `_wasInterruptedByNewMessage`
- Exported factory functions: `createStatusCallback()`, `createAskUserKeyboard()`, `createReadStream()`

**Variables:**
- camelCase for all variables: `userId`, `chatId`, `sessionId`, `childProcess`, `messageQueue`
- Constants use UPPER_SNAKE_CASE: `ALLOWED_PATHS`, `RATE_LIMIT_REQUESTS`, `TELEGRAM_MESSAGE_LIMIT`, `BUTTON_LABEL_MAX_LENGTH`
- Tuple destructuring with descriptive names: `[allowed, retryAfter]`, `[safe, reason]`, `[safe]`
- Map keys: lowercase with underscores: `lastEditTimes`, `lastContent`, `textMessages`

**Types:**
- PascalCase for interfaces: `RateLimitBucket`, `SavedSession`, `SessionHistory`, `TokenUsage`, `AuditEvent`, `PendingMediaGroup`
- PascalCase for type aliases: `StatusCallback`, `AuditEventType`, `BotContext`, `McpServerConfig`
- Union types use PascalCase members: `"thinking" | "tool" | "text" | "segment_end" | "done"`
- Exported types with `export type` and `export interface`

## Code Style

**Formatting:**
- No dedicated formatter configured (no `.prettierrc`, `biome.json`, or `eslint.config`)
- Consistent manual formatting observed: 2-space indentation, consistent quote usage
- Line length varies (80-120 columns observed)

**Linting:**
- No ESLint or other linter configured for the project
- TypeScript strict mode enabled in `tsconfig.json`:
  - `"strict": true`
  - `"noUncheckedIndexedAccess": true`
  - `"noImplicitOverride": true`
  - `"noFallthroughCasesInSwitch": true`
- Type checking enforced: `bun run typecheck` used before commits

**Module System:**
- ES modules: `"type": "module"` in `package.json`
- Import syntax: `import { name } from "path"` and `import type { Type } from "path"`
- Bundler module resolution: `"moduleResolution": "bundler"` in `tsconfig.json`

## Import Organization

**Order:**
1. Built-in Node.js modules: `import { spawn } from "child_process"`
2. Third-party dependencies: `import { describe, it, expect } from "vitest"`
3. Type imports: `import type { Context } from "grammy"`
4. Relative imports: `import { session } from "../session"`
5. Type imports from relative: `import type { SavedSession } from "./types"`

**Path Aliases:**
- No path aliases configured (`tsconfig.json` uses standard bundler resolution)
- All relative imports use explicit `../` paths: `import { config } from "../config"`
- Root context understood from file location

**Barrel Files:**
- Not commonly used; most files export specific named exports
- `src/handlers/index.ts` re-exports handler functions but not all modules use barrel pattern

## Error Handling

**Patterns:**
- Try-catch blocks used for operations that may fail: file I/O, JSON parsing, subprocess spawning
- Silent failures with fallback: `catch { return false }` or `catch { /* use default */ }`
- Error propagation for critical operations: throws from `setWorkingDir()`, `addProject()`
- Error description as tuple return: `[safe: boolean, reason: string]` from `checkCommandSafety()`
- Errors logged with context: `console.error()`, `console.warn()`, audit log entries

**Example patterns from `src/security.ts`:**
```typescript
export function isPathAllowed(path: string): boolean {
  try {
    // logic
    return false;
  } catch {
    return false;  // Safe default on any error
  }
}

export function checkCommandSafety(command: string): [safe: boolean, reason: string] {
  // Returns both success and reason for failure
  return [false, `Blocked pattern: ${pattern}`];
}
```

**Example from `src/session.ts`:**
```typescript
function killProcessTree(pid: number): void {
  try {
    if (process.platform === "win32") {
      execSync(`taskkill /pid ${pid} /T /F`, { stdio: "ignore" });
    } else {
      process.kill(-pid, "SIGTERM");
      setTimeout(() => {
        try {
          process.kill(-pid, "SIGKILL");
        } catch {
          // Already dead
        }
      }, 5000);
    }
  } catch {
    // Process might already be dead
  }
}
```

## Logging

**Framework:** `console` (no external logging library)

**Patterns:**
- Info messages: `console.log()` for startup/state changes
- Warnings: `console.warn()` for recoverable issues
- Errors: `console.error()` for failures
- Audit logging: separate `auditLog()` function writes to file with structured events

**From `src/config.ts`:**
```typescript
console.log(
  `Loaded ${Object.keys(MCP_SERVERS).length} MCP servers from mcp-config.ts`
);
console.log("No mcp-config.ts found - running without MCPs");
```

**From `src/session.ts`:**
```typescript
console.log(`Working directory changed to: ${path}`);
console.log(`Restored working directory: ${state.working_dir}`);
console.warn("Failed to save state:", err);
```

**Audit logging from `src/utils.ts`:**
```typescript
export async function auditLog(
  userId: number,
  username: string,
  messageType: string,
  content: string,
  response = ""
): Promise<void> {
  const event: AuditEvent = {
    timestamp: new Date().toISOString(),
    event: "message",
    user_id: userId,
    username,
    message_type: messageType,
    content,
  };
  await writeAuditLog(event);
}
```

## Comments

**When to Comment:**
- JSDoc blocks for exported functions and classes: document purpose, parameters, returns
- Inline comments for complex logic and non-obvious workarounds
- Section headers with `// ============== Section Name ==============` for module organization
- Platform-specific logic noted: "Windows" vs "Unix" branches
- TODO/FIXME comments not heavily used (not observed in codebase)

**JSDoc/TSDoc:**
- Used for public APIs in `types.ts`, `session.ts`, handlers
- Documents type definitions and function signatures
- Example from `src/types.ts`:
```typescript
/**
 * Shared TypeScript types for the Claude Telegram Bot.
 */

/**
 * Status callback for streaming updates
 */
export type StatusCallback = (
  type: "thinking" | "tool" | "text" | "segment_end" | "done",
  content: string,
  segmentId?: number
) => Promise<void>;
```

**Section Headers:**
- `// ============== SectionName ==============` used consistently to organize code blocks
- Found in: `session.ts`, `config.ts`, `security.ts`, `formatting.ts`, `streaming.ts`
- Creates visual separation for: Rate Limiter, Path Validation, Command Safety, etc.

## Function Design

**Size:** Functions are medium-length (10-50 lines typical), with longer functions reserved for complex workflows

**Parameters:**
- Use single Context parameter: `handleText(ctx: Context)`
- Tuple returns for multiple values: `[allowed: boolean, retryAfter?: number]`
- No function overloading; use optional parameters and union types
- Destructuring in function signature: `async function handleText(ctx: Context): Promise<void>`

**Return Values:**
- Explicit return types: all functions have return type annotations
- `Promise<void>` for async handlers; `Promise<T>` for async operations
- Tuple returns for functions that need to communicate multiple values: `[success, error]` or `[allowed, retryAfter]`
- `null` for missing state (e.g., `sessionId: string | null = null`)
- Early returns for guard clauses (authorization, validation checks)

**Example from `src/session.ts`:**
```typescript
get currentWorkingDir(): string {
  return this._workingDir;
}

setWorkingDir(path: string): void {
  if (!existsSync(path)) {
    throw new Error(`Directory does not exist: ${path}`);
  }
  this._workingDir = path;
  this.saveState();
}

check(userId: number): [allowed: boolean, retryAfter?: number] {
  if (!RATE_LIMIT_ENABLED) {
    return [true];
  }
  // ... logic ...
  return [false, retryAfter];
}
```

## Module Design

**Exports:**
- Named exports preferred: `export function handleText()`, `export class RateLimiter`
- Type exports use `export type`: `export type StatusCallback = ...`
- Single default export not used (consistent named exports throughout)

**Barrel Files:**
- `src/handlers/index.ts` imports handlers from submodules
- Not all modules follow barrel pattern; most require direct imports

**File Organization Pattern:**
1. File header comment documenting purpose
2. Type/interface definitions
3. Constants and configuration
4. Helper functions (private/implementation detail)
5. Main export classes/functions
6. Instance creation and export: `export const rateLimiter = new RateLimiter()`

**From `src/formatting.ts` (typical pattern):**
```typescript
/**
 * Formatting module for Claude Telegram Bot.
 *
 * Markdown conversion and tool status display formatting.
 */

/**
 * Escape HTML special characters.
 */
export function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

// ============== Helper ==============

function convertBlockquotes(text: string): string {
  // ...
}
```

---

*Convention analysis: 2026-03-19*
