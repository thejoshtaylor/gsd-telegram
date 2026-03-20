package handlers

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/user/gsd-tele-go/internal/config"
	"github.com/user/gsd-tele-go/internal/session"
)

// callbackAction is an enum for the action to take after parsing callback data.
// Using a pure enum lets us test routing logic without gotgbot types.
type callbackAction int

const (
	callbackActionUnknown    callbackAction = iota
	callbackActionResume                    // data = "resume:<session_id>"
	callbackActionStop                      // data = "action:stop"
	callbackActionNew                       // data = "action:new"
	callbackActionRetry                     // data = "action:retry"
)

// parseCallbackData parses the callback data string and returns the action and
// optional payload (e.g. session ID for resume).
// Pure function — no Telegram types — fully testable.
func parseCallbackData(data string) (callbackAction, string) {
	switch {
	case strings.HasPrefix(data, "resume:"):
		return callbackActionResume, strings.TrimPrefix(data, "resume:")
	case data == "action:stop":
		return callbackActionStop, ""
	case data == "action:new":
		return callbackActionNew, ""
	case data == "action:retry":
		return callbackActionRetry, ""
	default:
		return callbackActionUnknown, ""
	}
}

// HandleCallback handles all inline keyboard callback queries.
//
// Routes by callback data prefix:
//   - "resume:<session_id>" — restore a persisted Claude session
//   - "action:stop"         — stop the current running query
//   - "action:new"          — start a fresh session
//   - "action:retry"        — placeholder (deferred to v2)
func HandleCallback(b *gotgbot.Bot, ctx *ext.Context, store *session.SessionStore, persist *session.PersistenceManager, cfg *config.Config) error {
	if ctx.CallbackQuery == nil {
		return nil
	}

	cq := ctx.CallbackQuery

	// Answer the callback query immediately to remove the loading spinner.
	if _, err := cq.Answer(b, nil); err != nil {
		// Non-fatal: the user experience degrades slightly but we continue.
		_ = err
	}

	// Determine the chat ID from the message attached to the callback query.
	var chatID int64
	var msgID int64
	if cq.Message != nil {
		chat := cq.Message.GetChat()
		chatID = chat.Id
		msgID = cq.Message.GetMessageId()
	}

	if chatID == 0 {
		return nil
	}

	action, payload := parseCallbackData(cq.Data)

	switch action {
	case callbackActionResume:
		return handleCallbackResume(b, ctx, store, cfg, chatID, msgID, payload)

	case callbackActionStop:
		return handleCallbackStop(b, store, chatID, msgID)

	case callbackActionNew:
		return handleCallbackNew(b, store, cfg, chatID, msgID)

	case callbackActionRetry:
		// Deferred to v2.
		_, err := b.SendMessage(chatID, "Retry not available yet.", nil)
		return err

	default:
		// Unknown callback data — log and ignore gracefully.
		return nil
	}
}

// handleCallbackResume restores a saved session by session ID.
// Updates the inline keyboard message to confirm the restore.
func handleCallbackResume(b *gotgbot.Bot, _ *ext.Context, store *session.SessionStore, cfg *config.Config, chatID, msgID int64, sessionID string) error {
	if sessionID == "" {
		_, err := b.SendMessage(chatID, "Invalid session ID.", nil)
		return err
	}

	sess := store.GetOrCreate(chatID, cfg.WorkingDir)
	sess.SetSessionID(sessionID)

	// Show a short prefix of the session ID for confirmation.
	short := sessionID
	if len(short) > 8 {
		short = short[:8]
	}

	confirmText := fmt.Sprintf("Session restored: %s... — send a message to continue.", short)

	// Edit the keyboard message to remove the buttons and show the confirmation.
	if msgID != 0 {
		_, _, _ = b.EditMessageText(confirmText, &gotgbot.EditMessageTextOpts{
			ChatId:    chatID,
			MessageId: msgID,
		})
	} else {
		_, _ = b.SendMessage(chatID, confirmText, nil)
	}

	return nil
}

// handleCallbackStop stops the current running query (same as /stop).
func handleCallbackStop(b *gotgbot.Bot, store *session.SessionStore, chatID, msgID int64) error {
	sess, ok := store.Get(chatID)
	if !ok || !sess.IsRunning() {
		_, err := b.SendMessage(chatID, "No query running.", nil)
		return err
	}

	sess.Stop()

	if msgID != 0 {
		_, _, _ = b.EditMessageText("Query stopped.", &gotgbot.EditMessageTextOpts{
			ChatId:    chatID,
			MessageId: msgID,
		})
	} else {
		_, _ = b.SendMessage(chatID, "Query stopped.", nil)
	}

	return nil
}

// handleCallbackNew starts a fresh session (same as /new).
func handleCallbackNew(b *gotgbot.Bot, store *session.SessionStore, cfg *config.Config, chatID, msgID int64) error {
	sess := store.GetOrCreate(chatID, cfg.WorkingDir)

	if sess.IsRunning() {
		sess.Stop()
	}

	sess.SetSessionID("")

	if msgID != 0 {
		_, _, _ = b.EditMessageText("New session started. Previous session saved for /resume.", &gotgbot.EditMessageTextOpts{
			ChatId:    chatID,
			MessageId: msgID,
		})
	} else {
		_, _ = b.SendMessage(chatID, "New session started. Previous session saved for /resume.", nil)
	}

	return nil
}
