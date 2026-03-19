# Testing Patterns

**Analysis Date:** 2026-03-19

## Test Framework

**Runner:**
- Vitest 4.0.18
- Config: `vitest.config.ts`

**Assertion Library:**
- Vitest built-in expect() API (compatible with Jest)

**Run Commands:**
```bash
bun run test        # Run all tests
bun run test:watch # Watch mode for continuous testing
```

**Config file location:** `vitest.config.ts`
```typescript
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["tests/**/*.test.ts"],
  },
});
```

## Test File Organization

**Location:**
- Co-located in `tests/` directory at root level
- Not alongside source files (separate from `src/`)

**Naming:**
- Pattern: `[module-name].test.ts`
- Examples: `security.test.ts`, `commands.test.ts`, `formatting.test.ts`, `registry.test.ts`

**File Structure:**
```
tests/
├── security.test.ts    # Tests for src/security.ts
├── commands.test.ts    # Tests for src/handlers/commands.ts
├── formatting.test.ts  # Tests for src/formatting.ts
└── registry.test.ts    # Tests for src/registry.ts
```

**Current Test Coverage:**
- 4 test files covering core modules
- 80+ test cases total
- Focus areas: security, formatting, command parsing, project registry

## Test Structure

**Suite Organization:**
```typescript
import { describe, it, expect, vi, beforeEach } from "vitest";

describe("moduleName", () => {
  it("does something expected", () => {
    expect(result).toBe(expected);
  });

  it("handles edge case", () => {
    expect(result).toEqual(expectedObject);
  });
});
```

**Describe Grouping:**
Tests grouped by function name with comment separators:

From `tests/security.test.ts`:
```typescript
// ============== isAuthorized ==============

describe("isAuthorized", () => {
  it("allows an authorized user", () => {
    expect(isAuthorized(12345, [12345, 67890])).toBe(true);
  });

  it("rejects an unauthorized user", () => {
    expect(isAuthorized(99999, [12345, 67890])).toBe(false);
  });
});

// ============== checkCommandSafety ==============

describe("checkCommandSafety", () => {
  it("allows safe commands", () => {
    expect(checkCommandSafety("git status")[0]).toBe(true);
  });
});
```

**Patterns:**
- One `describe()` per exported function/class
- Section separators with `// ============== Name ==============`
- Flat test structure (no nested describe blocks observed)
- `beforeEach()` for test setup when needed
- Meaningful test names describing behavior, not implementation

## Mocking

**Framework:** Vitest `vi` mock utilities

**Patterns:**
Mock modules before importing code that uses them:

From `tests/security.test.ts`:
```typescript
// Mock config BEFORE importing security module
vi.mock("../src/config", () => ({
  ALLOWED_PATHS: ["D:/Projects", "D:/___ATM", "C:/Users/TestUser/Documents"],
  BLOCKED_PATTERNS: ["rm -rf /", "rm -rf ~", ...],
  RATE_LIMIT_ENABLED: true,
  RATE_LIMIT_REQUESTS: 5,
  RATE_LIMIT_WINDOW: 60,
  TEMP_PATHS: ["C:/Temp", "D:/Temp"],
}));

import { isAuthorized, checkCommandSafety, RateLimiter } from "../src/security";
```

From `tests/commands.test.ts`:
```typescript
// Mock multiple dependencies
vi.mock("../src/config", () => ({ ALLOWED_USERS: [12345] }));
vi.mock("../src/session", () => ({ session: {} }));
vi.mock("../src/utils", () => ({ sleep: vi.fn() }));
vi.mock("../src/registry", () => ({ parseRegistry: vi.fn(() => []) }));
vi.mock("../src/security", () => ({ isAuthorized: vi.fn(() => false) }));

vi.mock("fs", async () => {
  const actual = await vi.importActual<typeof import("fs")>("fs");
  return {
    ...actual,
    existsSync: vi.fn(),
    readFileSync: vi.fn(),
    writeFileSync: vi.fn(),
  };
});
```

**Accessing Mocks:**
```typescript
const mockExistsSync = vi.mocked(existsSync);
const mockReadFileSync = vi.mocked(readFileSync);

// Use in test
mockExistsSync.mockReturnValue(true);
mockReadFileSync.mockReturnValue("file content");
```

**What to Mock:**
- Config modules (environment variables, paths, constants)
- File system operations (existsSync, readFileSync, writeFileSync)
- External dependencies (OpenAI, etc.)
- Utility functions that are irrelevant to the test
- Session/state management for isolation

**What NOT to Mock:**
- Functions being tested (test the real implementation)
- Pure functions with no side effects (no mock needed)
- Logic in the function under test (test real behavior)

## Fixtures and Factories

**Test Data:**
From `tests/registry.test.ts`:
```typescript
const FIXTURE = `# Project Registry

| Name | Type | Status | Location | Description |
|------|------|--------|----------|-------------|
| AlphaProject | App | Active | D:\\Projects\\Alpha | Main application |
| BetaScript | Script | Archived | D:\\___ATM\\Beta | Utility script |
| GammaLib | Library | Active | D:\\Projects\\Gamma | Shared library |
| DeltaTool | Tool | Paused | D:\\Projects\\Delta | Dev tooling |

## Definitions
Status values...
`;
```

**Location:**
- Fixtures defined inline in test files as string constants
- Named with uppercase: `FIXTURE`, `TEST_DATA`
- No separate fixtures directory

**Usage Pattern:**
```typescript
describe("parseRegistry", () => {
  it("parses all rows from the table", () => {
    mockReadFileSync.mockReturnValue(FIXTURE);
    const projects = parseRegistry();
    expect(projects).toHaveLength(4);
  });
});
```

## Coverage

**Requirements:** Not enforced (no coverage configuration in `vitest.config.ts`)

**View Coverage:**
```bash
# No coverage reports configured
# To add coverage, would need:
# bun run test -- --coverage
```

**Current State:**
- No coverage thresholds enforced
- No CI integration for coverage checks
- Tests written for critical paths (security, parsing, formatting)

## Test Types

**Unit Tests:**
- Scope: Individual functions in isolation with mocked dependencies
- Approach: Test function inputs and outputs, verify return values
- Examples: `isAuthorized()`, `checkCommandSafety()`, `escapeHtml()`

From `tests/formatting.test.ts`:
```typescript
describe("escapeHtml", () => {
  it("escapes ampersands", () => {
    expect(escapeHtml("foo & bar")).toBe("foo &amp; bar");
  });

  it("escapes angle brackets", () => {
    expect(escapeHtml("<script>alert(1)</script>")).toBe(
      "&lt;script&gt;alert(1)&lt;/script&gt;"
    );
  });
});
```

**Integration Tests:**
- Scope: Multiple components together (e.g., RateLimiter with time tracking)
- Approach: Test state changes, interactions, time-dependent behavior

From `tests/security.test.ts`:
```typescript
describe("RateLimiter", () => {
  it("maintains independent buckets per user", () => {
    const limiter = new RateLimiter();

    // Exhaust user 1's tokens
    for (let i = 0; i < 5; i++) {
      limiter.check(1);
    }
    expect(limiter.check(1)[0]).toBe(false);

    // User 2 should still have full quota
    const [allowed] = limiter.check(2);
    expect(allowed).toBe(true);
  });

  it("refills tokens over time", () => {
    vi.useFakeTimers();
    const limiter = new RateLimiter();

    // Exhaust all tokens
    for (let i = 0; i < 5; i++) {
      limiter.check(1);
    }

    // Advance time for token refill
    vi.advanceTimersByTime(13_000);
    const [allowed] = limiter.check(1);
    expect(allowed).toBe(true);

    vi.useRealTimers();
  });
});
```

**E2E Tests:**
- Status: Not used
- No end-to-end tests for bot interactions with Telegram
- Tests focus on library functions and parsing logic

## Common Patterns

**Async Testing:**
Not heavily used in current test suite. Pattern would be:
```typescript
it("handles async operation", async () => {
  const result = await asyncFunction();
  expect(result).toBeDefined();
});
```

**Error Testing:**
Tests verify tuple return indicating error:
```typescript
it("blocks rm -rf /", () => {
  const [safe, reason] = checkCommandSafety("rm -rf /");
  expect(safe).toBe(false);
  expect(reason).toContain("rm -rf /");
});
```

**Parametrized Tests:**
Not used. Instead, separate test case per scenario:
```typescript
it("allows safe commands", () => {
  expect(checkCommandSafety("git status")[0]).toBe(true);
  expect(checkCommandSafety("npm install")[0]).toBe(true);
  expect(checkCommandSafety("ls -la")[0]).toBe(true);
});
```

**State Management Testing:**
```typescript
describe("RateLimiter", () => {
  beforeEach(() => {
    vi.useRealTimers();  // Reset timer state
  });

  it("gets status", () => {
    const limiter = new RateLimiter();
    const status = limiter.getStatus(999);
    expect(status.max).toBe(5);
    expect(status.tokens).toBe(5);
    expect(status.refillRate).toBeCloseTo(5 / 60);
  });
});
```

## Test Execution

**Run All Tests:**
```bash
bun run test
```

**Watch Mode:**
```bash
bun run test:watch
```

**Type Checking Before Tests:**
```bash
bun run typecheck
```

**Workflow:**
- Run typecheck first to catch type errors
- Run tests to verify logic
- Watch mode during development for continuous feedback

## Assertion Patterns

**Boolean Assertions:**
```typescript
expect(isAuthorized(12345, [12345, 67890])).toBe(true);
expect(checkCommandSafety("git status")[0]).toBe(true);
```

**Equality Assertions:**
```typescript
expect(phases).toHaveLength(1);
expect(projects).toEqual([]);
expect(projects[0]!.number).toBe("4");
```

**Object Assertions:**
```typescript
expect(phases[0]).toEqual({
  number: "4",
  name: "Dashboard",
  description: "Web UI for monitoring",
  status: "done",
});
```

**Collection Assertions:**
```typescript
expect(GSD_OPERATIONS).toHaveLength(16);
expect(new Set(keys).size).toBe(keys.length);  // No duplicates
```

**Type Assertions:**
```typescript
expect(status.refillRate).toBeCloseTo(5 / 60);
expect(retryAfter).toBeGreaterThan(0);
expect(retryAfter).toBeLessThan(60);
```

**Matcher Patterns:**
```typescript
expect(result).toContain("some string");
expect(result).toMatch(/^\/gsd:/);
expect(result).not.toContain("<script>");
expect(label).toBeDefined();
```

---

*Testing analysis: 2026-03-19*
