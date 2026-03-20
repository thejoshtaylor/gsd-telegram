package bot

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"

	bothandlers "github.com/user/gsd-tele-go/internal/handlers"
)

// registerHandlers wires up the full middleware and handler chain on dispatcher.
//
// Group ordering:
//   - Group -2: auth middleware (runs first; rejects unauthorized users)
//   - Group -1: rate limit middleware (runs second; throttles per-channel)
//   - Group  0: message and command handlers
func (b *Bot) registerHandlers(dispatcher *ext.Dispatcher) {
	// --- Middleware groups ---
	// Build a placeholder terminal handler that the middleware delegates to.
	// The middleware wraps the *next* handler; we chain them manually.
	// In gotgbot, middleware is registered as handlers in lower-numbered groups.

	// Auth middleware: wraps a no-op so that after passing auth, processing falls
	// through to group -1 and then 0.
	authPass := &passthroughHandler{name: "auth_pass"}
	dispatcher.AddHandlerToGroup(b.authMiddleware(authPass), -2)

	// Rate limit middleware.
	rateLimitPass := &passthroughHandler{name: "rate_limit_pass"}
	dispatcher.AddHandlerToGroup(b.rateLimitMiddleware(rateLimitPass), -1)

	// --- Text message handler ---
	dispatcher.AddHandler(handlers.NewMessage(message.Text, b.handleText))

	// --- Command handlers (placeholder implementations) ---
	dispatcher.AddHandler(handlers.NewCommand("start", b.handleStart))
	dispatcher.AddHandler(handlers.NewCommand("new", b.handleNew))
	dispatcher.AddHandler(handlers.NewCommand("stop", b.handleStop))
	dispatcher.AddHandler(handlers.NewCommand("status", b.handleStatus))
	dispatcher.AddHandler(handlers.NewCommand("resume", b.handleResume))
}

// handleText is the bot-layer wrapper that calls the handlers.HandleText function.
func (b *Bot) handleText(tgBot *gotgbot.Bot, ctx *ext.Context) error {
	return bothandlers.HandleText(tgBot, ctx, b.store, b.cfg, b.auditLog, b.persist, b.WaitGroup())
}

// --- Placeholder command handlers ---
// These are stub implementations that will be replaced in Plan 07 (command handlers).

func (b *Bot) handleStart(tgBot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage != nil {
		_, _ = ctx.EffectiveMessage.Reply(tgBot, "Bot is running. Send a message to start.", nil)
	}
	return nil
}

func (b *Bot) handleNew(tgBot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage != nil {
		_, _ = ctx.EffectiveMessage.Reply(tgBot, "/new — not yet implemented.", nil)
	}
	return nil
}

func (b *Bot) handleStop(tgBot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage != nil {
		_, _ = ctx.EffectiveMessage.Reply(tgBot, "/stop — not yet implemented.", nil)
	}
	return nil
}

func (b *Bot) handleStatus(tgBot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage != nil {
		_, _ = ctx.EffectiveMessage.Reply(tgBot, "/status — not yet implemented.", nil)
	}
	return nil
}

func (b *Bot) handleResume(tgBot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage != nil {
		_, _ = ctx.EffectiveMessage.Reply(tgBot, "/resume — not yet implemented.", nil)
	}
	return nil
}

// passthroughHandler is a no-op handler used as the terminal target for middleware
// chains. When the middleware calls next.HandleUpdate(), this handler returns nil
// allowing the update to fall through to the next dispatcher group.
type passthroughHandler struct {
	name string
}

func (p *passthroughHandler) CheckUpdate(_ *gotgbot.Bot, _ *ext.Context) bool {
	return true
}

func (p *passthroughHandler) HandleUpdate(_ *gotgbot.Bot, _ *ext.Context) error {
	return nil
}

func (p *passthroughHandler) Name() string {
	return p.name
}
