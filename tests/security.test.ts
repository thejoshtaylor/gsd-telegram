import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock config before importing security module
vi.mock("../src/config", () => ({
  ALLOWED_PATHS: ["D:/Projects", "D:/___ATM", "C:/Users/TestUser/Documents"],
  BLOCKED_PATTERNS: [
    "rm -rf /",
    "rm -rf ~",
    "rm -rf $HOME",
    "rm -rf %USERPROFILE%",
    "sudo rm",
    ":(){ :|:& };:",
    "> /dev/sd",
    "mkfs.",
    "dd if=",
    "format c:",
    "del /s /q c:",
  ],
  RATE_LIMIT_ENABLED: true,
  RATE_LIMIT_REQUESTS: 5,
  RATE_LIMIT_WINDOW: 60,
  TEMP_PATHS: ["C:/Temp", "D:/Temp"],
}));

import { isAuthorized, checkCommandSafety, RateLimiter } from "../src/security";

// ============== isAuthorized ==============

describe("isAuthorized", () => {
  it("allows an authorized user", () => {
    expect(isAuthorized(12345, [12345, 67890])).toBe(true);
  });

  it("rejects an unauthorized user", () => {
    expect(isAuthorized(99999, [12345, 67890])).toBe(false);
  });

  it("rejects undefined userId", () => {
    expect(isAuthorized(undefined, [12345])).toBe(false);
  });

  it("rejects when allowed list is empty", () => {
    expect(isAuthorized(12345, [])).toBe(false);
  });
});

// ============== checkCommandSafety ==============

describe("checkCommandSafety", () => {
  it("allows safe commands", () => {
    expect(checkCommandSafety("git status")[0]).toBe(true);
    expect(checkCommandSafety("npm install")[0]).toBe(true);
    expect(checkCommandSafety("ls -la")[0]).toBe(true);
  });

  it("blocks rm -rf /", () => {
    const [safe, reason] = checkCommandSafety("rm -rf /");
    expect(safe).toBe(false);
    expect(reason).toContain("rm -rf /");
  });

  it("blocks fork bomb", () => {
    const [safe] = checkCommandSafety(":(){ :|:& };:");
    expect(safe).toBe(false);
  });

  it("blocks sudo rm", () => {
    const [safe] = checkCommandSafety("sudo rm -rf /var");
    expect(safe).toBe(false);
  });

  it("blocks mkfs", () => {
    const [safe] = checkCommandSafety("mkfs.ext4 /dev/sda1");
    expect(safe).toBe(false);
  });

  it("blocks dd if=", () => {
    const [safe] = checkCommandSafety("dd if=/dev/zero of=/dev/sda");
    expect(safe).toBe(false);
  });

  it("blocks format c:", () => {
    const [safe] = checkCommandSafety("format c:");
    expect(safe).toBe(false);
  });

  it("blocks case-insensitively", () => {
    const [safe] = checkCommandSafety("FORMAT C:");
    expect(safe).toBe(false);
  });

  it("blocks rm -rf ~", () => {
    const [safe] = checkCommandSafety("rm -rf ~");
    expect(safe).toBe(false);
  });

  it("blocks > /dev/sd", () => {
    const [safe] = checkCommandSafety("cat something > /dev/sda");
    expect(safe).toBe(false);
  });

  it("blocks del /s /q c:", () => {
    const [safe] = checkCommandSafety("del /s /q c:");
    expect(safe).toBe(false);
  });
});

// ============== RateLimiter ==============

describe("RateLimiter", () => {
  beforeEach(() => {
    vi.useRealTimers();
  });

  it("allows requests within limit", () => {
    const limiter = new RateLimiter();
    const [allowed] = limiter.check(1);
    expect(allowed).toBe(true);
  });

  it("blocks after exceeding limit", () => {
    const limiter = new RateLimiter();
    // Exhaust all 5 tokens
    for (let i = 0; i < 5; i++) {
      const [allowed] = limiter.check(1);
      expect(allowed).toBe(true);
    }
    // 6th request should be blocked
    const [allowed, retryAfter] = limiter.check(1);
    expect(allowed).toBe(false);
    expect(retryAfter).toBeGreaterThan(0);
  });

  it("refills tokens over time", () => {
    vi.useFakeTimers();
    const limiter = new RateLimiter();

    // Exhaust all tokens
    for (let i = 0; i < 5; i++) {
      limiter.check(1);
    }
    expect(limiter.check(1)[0]).toBe(false);

    // Advance time enough for 1 token to refill
    // refillRate = 5/60 tokens/sec, need 1 token → 12 seconds
    vi.advanceTimersByTime(13_000);

    const [allowed] = limiter.check(1);
    expect(allowed).toBe(true);
  });

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

  it("getStatus returns correct values", () => {
    const limiter = new RateLimiter();
    const status = limiter.getStatus(999);
    expect(status.max).toBe(5);
    expect(status.tokens).toBe(5); // full bucket for unknown user
    expect(status.refillRate).toBeCloseTo(5 / 60);
  });

  it("retryAfter is positive when rate limited", () => {
    const limiter = new RateLimiter();
    for (let i = 0; i < 5; i++) {
      limiter.check(1);
    }
    const [, retryAfter] = limiter.check(1);
    expect(retryAfter).toBeGreaterThan(0);
    expect(retryAfter).toBeLessThan(60); // should be well under the full window
  });
});
