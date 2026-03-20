package handlers

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/rs/zerolog/log"

	"github.com/user/gsd-tele-go/internal/audit"
	"github.com/user/gsd-tele-go/internal/claude"
	"github.com/user/gsd-tele-go/internal/config"
	"github.com/user/gsd-tele-go/internal/security"
	"github.com/user/gsd-tele-go/internal/session"
)

// HandleText processes a text message, routing it to the channel's Claude session.
//
// The wg parameter is the bot's WaitGroup.  HandleText calls wg.Add(1) when starting
// a new session worker goroutine, ensuring the Bot tracks all active workers for
// graceful shutdown.
func HandleText(
	tgBot *gotgbot.Bot,
	ctx *ext.Context,
	store *session.SessionStore,
	cfg *config.Config,
	auditLog *audit.Logger,
	persist *session.PersistenceManager,
	wg *sync.WaitGroup,
) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}

	text := ctx.EffectiveMessage.Text
	chatID := ctx.EffectiveChat.Id

	var userID int64
	if ctx.EffectiveSender != nil {
		userID = ctx.EffectiveSender.Id()
	}

	// Audit log the incoming message.
	if auditLog != nil {
		ev := audit.NewEvent("message", userID, chatID)
		excerpt := text
		if len(excerpt) > 100 {
			excerpt = excerpt[:100]
		}
		ev.Message = excerpt
		_ = auditLog.Log(ev)
	}

	// --- Interrupt handling ---
	// A message prefixed with "!" interrupts the running query and uses the
	// stripped text as the new message.
	sess := store.GetOrCreate(chatID, cfg.WorkingDir)
	if strings.HasPrefix(text, "!") {
		stripped := strings.TrimSpace(text[1:])
		if sess.IsRunning() {
			sess.MarkInterrupt()
			sess.Stop()
		}
		text = stripped
	}

	// Empty message after stripping — ignore.
	if text == "" {
		return nil
	}

	// --- Command safety check ---
	safe, blockedPattern := security.CheckCommandSafety(text, config.BlockedPatterns)
	if !safe {
		log.Warn().
			Int64("chat_id", chatID).
			Int64("user_id", userID).
			Str("pattern", blockedPattern).
			Msg("Blocked message due to safety pattern")
		_, err := ctx.EffectiveMessage.Reply(tgBot, "Command blocked for safety: "+blockedPattern, nil)
		return err
	}

	// --- Ensure session worker is running ---
	// A new worker is started if the session was just created or has no running worker.
	// We use context.Background() so the worker is not tied to the per-update goroutine.
	// Shutdown is handled via the session's stop channel and WaitGroup.
	capturedText := text
	capturedChatID := chatID
	capturedUserID := userID

	workerCfg := session.WorkerConfig{
		AllowedPaths: cfg.AllowedPaths,
		SafetyPrompt: cfg.SafetyPrompt,
		FilteredEnv:  config.FilteredEnv(),
		OnQueryComplete: func(sessionID string) {
			// Persist the session after each successful query.
			title := capturedText
			if len(title) > 50 {
				title = title[:50]
			}
			saved := session.SavedSession{
				SessionID:  sessionID,
				SavedAt:    time.Now().UTC().Format(time.RFC3339),
				WorkingDir: sess.WorkingDir(),
				Title:      title,
				ChannelID:  capturedChatID,
			}
			if err := persist.Save(saved); err != nil {
				log.Warn().Err(err).Msg("Failed to persist session")
			}
		},
	}

	// Check if session already has an active worker. We start one if the session
	// was freshly created (not running) by looking at the queue state.
	// A worker must be started before enqueueing so the message doesn't sit forever.
	// The double-start is safe: Worker() runs its own loop; if the worker is already
	// running (because this session was restored), the new goroutine will simply block
	// on the queue. But to avoid double-goroutines we track with IsRunning.
	//
	// Actually: Worker is always started exactly once per new session. The session
	// IsRunning() refers to an active query, not whether the worker goroutine exists.
	// We use a mutex-free heuristic: start the worker if the session was just fetched
	// from GetOrCreate but has no session ID and no previous activity.
	// The safe approach: always start a worker; use a sync.Once inside Worker or
	// ensure only one goroutine calls Worker per session.
	//
	// For correctness we use a workerStarted channel pattern stored in the session.
	// Since the session doesn't have this, we ensure the worker is started at most once
	// by having the bot layer (restoreSessions) start workers for restored sessions,
	// and here we start for new sessions only.
	//
	// Detection: if SessionID is empty AND no query has run (StartedAt is recent
	// within 1 second), we start the worker.
	shouldStartWorker := sess.SessionID() == "" && time.Since(sess.StartedAt()) < time.Second

	if shouldStartWorker {
		wg.Add(1)
		go func(s *session.Session, c session.WorkerConfig) {
			defer wg.Done()
			s.Worker(context.Background(), cfg.ClaudeCLIPath, c)
		}(sess, workerCfg)
	}

	// --- Create streaming state and callback ---
	ss := NewStreamingState(tgBot, chatID)

	// Start typing indicator.
	typingCtl := StartTypingIndicator(tgBot, chatID)

	// Build the queued message with a callback factory.
	qMsg := session.QueuedMessage{
		Text:   text,
		ChatID: chatID,
		UserID: userID,
		Callback: func(_ int64) claude.StatusCallback {
			// Stop typing indicator once streaming begins.
			typingCtl.Stop()
			return CreateStatusCallback(ss)
		},
		ErrCh: make(chan error, 1),
	}

	// Enqueue the message (non-blocking).
	if !sess.Enqueue(qMsg) {
		_, err := ctx.EffectiveMessage.Reply(tgBot,
			"Queue full, please wait for the current query to finish.", nil)
		typingCtl.Stop()
		return err
	}

	// Process the error channel asynchronously so we don't block the handler.
	go func() {
		err, ok := <-qMsg.ErrCh
		if !ok || err == nil {
			return
		}

		errStr := err.Error()

		// Context limit: session was auto-cleared, notify user.
		if strings.Contains(strings.ToLower(errStr), "context limit") {
			_, sendErr := tgBot.SendMessage(chatID,
				"Session hit hard context limit and was cleared. Use /resume to restore a previous session.",
				nil)
			if sendErr != nil {
				log.Warn().Err(sendErr).Msg("Failed to send context limit message")
			}
			return
		}

		// General Claude error: truncate stderr to 200 chars.
		truncated := errStr
		if len(truncated) > 200 {
			truncated = truncated[:200]
		}
		_, sendErr := tgBot.SendMessage(chatID, "Claude error: "+truncated, nil)
		if sendErr != nil {
			log.Warn().Err(sendErr).Str("original_error", errStr).Msg("Failed to send error message")
		}

		// Log the error.
		if auditLog != nil {
			ev := audit.NewEvent("claude_error", capturedUserID, capturedChatID)
			ev.Error = truncated
			ev.Message = capturedText
			_ = auditLog.Log(ev)
		}
	}()

	return nil
}
