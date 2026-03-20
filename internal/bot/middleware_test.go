package bot

import (
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// --- Mock implementations ---

// mockAuthChecker implements AuthChecker with a fixed response.
type mockAuthChecker struct {
	authorized bool
	calledWith struct {
		userID    int64
		channelID int64
	}
}

func (m *mockAuthChecker) IsAuthorized(userID int64, channelID int64) bool {
	m.calledWith.userID = userID
	m.calledWith.channelID = channelID
	return m.authorized
}

// mockRateLimitChecker implements RateLimitChecker with configurable behavior.
type mockRateLimitChecker struct {
	callCount int
	maxAllow  int // number of requests to allow before throttling
	delay     time.Duration
}

func (m *mockRateLimitChecker) Allow(_ int64) (bool, time.Duration) {
	m.callCount++
	if m.callCount <= m.maxAllow {
		return true, 0
	}
	return false, m.delay
}

// callTracker tracks whether the next handler was called.
type callTracker struct {
	called bool
}

func (c *callTracker) CheckUpdate(_ *gotgbot.Bot, _ *ext.Context) bool { return true }
func (c *callTracker) HandleUpdate(_ *gotgbot.Bot, _ *ext.Context) error {
	c.called = true
	return nil
}
func (c *callTracker) Name() string { return "call_tracker" }

// --- Test helpers ---

// buildContext creates a minimal ext.Context with the given userID and chatID.
// It works without a live Telegram connection by constructing the structs directly.
func buildContext(userID, chatID int64) *ext.Context {
	user := &gotgbot.User{Id: userID, FirstName: "Test"}
	chat := &gotgbot.Chat{Id: chatID}
	msg := &gotgbot.Message{
		MessageId: 1,
		From:      user,
		Chat:      *chat,
		Text:      "hello",
	}
	sender := &gotgbot.Sender{User: user}

	// ext.Context embeds *gotgbot.Update; we set EffectiveMessage etc. directly.
	return &ext.Context{
		Update:           &gotgbot.Update{Message: msg},
		EffectiveUser:    user,
		EffectiveChat:    chat,
		EffectiveMessage: msg,
		EffectiveSender:  sender,
	}
}

// --- Auth middleware tests ---

// TestMiddlewareAuthRejectsUnauthorized verifies that authMiddleware stops
// processing (does not call the next handler) for unauthorized users.
func TestMiddlewareAuthRejectsUnauthorized(t *testing.T) {
	checker := &mockAuthChecker{authorized: false}
	next := &callTracker{}

	mw := authMiddlewareWith(checker, nil, next)

	ctx := buildContext(999, 12345)
	err := mw.HandleUpdate(nil, ctx)

	if err != nil && err != ext.EndGroups {
		t.Fatalf("unexpected error: %v", err)
	}
	if next.called {
		t.Error("expected next handler NOT to be called for unauthorized user, but it was called")
	}
}

// TestMiddlewareAuthAllowsAuthorized verifies that authMiddleware calls the next
// handler when the user is authorized.
func TestMiddlewareAuthAllowsAuthorized(t *testing.T) {
	checker := &mockAuthChecker{authorized: true}
	next := &callTracker{}

	mw := authMiddlewareWith(checker, nil, next)

	ctx := buildContext(111, 12345)
	if err := mw.HandleUpdate(nil, ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !next.called {
		t.Error("expected next handler to be called for authorized user, but it was not")
	}
}

// TestMiddlewareAuthPassesChannelID verifies that authMiddleware passes the chat
// ID (channelID) to IsAuthorized — confirming Phase 2 forward-compat wiring.
func TestMiddlewareAuthPassesChannelID(t *testing.T) {
	const wantChannelID int64 = 99887766

	checker := &mockAuthChecker{authorized: true}
	next := &callTracker{}

	mw := authMiddlewareWith(checker, nil, next)

	ctx := buildContext(111, wantChannelID)
	if err := mw.HandleUpdate(nil, ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if checker.calledWith.channelID != wantChannelID {
		t.Errorf("authMiddleware passed channelID=%d to IsAuthorized; want %d",
			checker.calledWith.channelID, wantChannelID)
	}
}

// --- Rate limit middleware tests ---

// TestMiddlewareRateLimitThrottles verifies that after maxAllow requests, the
// next request is rejected with an EndGroups error (the reply function is skipped
// since we pass a nil bot, but the next handler must NOT be called).
func TestMiddlewareRateLimitThrottles(t *testing.T) {
	limiter := &mockRateLimitChecker{maxAllow: 2, delay: 30 * time.Second}
	next := &callTracker{}

	mw := rateLimitMiddlewareWith(true, limiter, nil, next)

	ctx := buildContext(111, 12345)

	// First two requests should pass through.
	for i := 0; i < 2; i++ {
		next.called = false
		if err := mw.HandleUpdate(nil, ctx); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i+1, err)
		}
		if !next.called {
			t.Errorf("request %d: expected next handler to be called but it was not", i+1)
		}
	}

	// Third request should be throttled.
	next.called = false
	err := mw.HandleUpdate(nil, ctx)
	if err != nil && err != ext.EndGroups {
		t.Fatalf("request 3: unexpected error: %v", err)
	}
	if next.called {
		t.Error("request 3: expected next handler NOT to be called when rate limited, but it was")
	}
}

// TestMiddlewareRateLimitDisabled verifies that when rate limiting is disabled,
// all requests pass through regardless of volume.
func TestMiddlewareRateLimitDisabled(t *testing.T) {
	// A limiter that would throttle immediately — but it should never be consulted.
	limiter := &mockRateLimitChecker{maxAllow: 0, delay: 60 * time.Second}
	next := &callTracker{}

	mw := rateLimitMiddlewareWith(false, limiter, nil, next)

	ctx := buildContext(111, 12345)

	for i := 0; i < 5; i++ {
		next.called = false
		if err := mw.HandleUpdate(nil, ctx); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i+1, err)
		}
		if !next.called {
			t.Errorf("request %d: expected next to be called when rate limiting disabled", i+1)
		}
	}

	if limiter.callCount > 0 {
		t.Errorf("rate limiter Allow() was called %d times when rate limiting is disabled; expected 0",
			limiter.callCount)
	}
}
