// Package handlers provides gotgbot handler functions for the gsd-tele-go bot.
//
// Each exported function corresponds to one bot command or message type.
// Handlers receive a *gotgbot.Bot and *ext.Context and return an error.
package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/user/gsd-tele-go/internal/config"
	"github.com/user/gsd-tele-go/internal/session"
)

// HandleStart handles the /start command (CMD-01).
// Shows bot welcome, version, current project path, and list of commands.
func HandleStart(b *gotgbot.Bot, ctx *ext.Context, store *session.SessionStore, cfg *config.Config) error {
	chatID := ctx.EffectiveChat.Id

	// Get session state for status display.
	sess, hasSess := store.Get(chatID)

	var stateStr string
	if !hasSess || sess.SessionID() == "" {
		stateStr = "No active session"
	} else if sess.IsRunning() {
		stateStr = "Running"
	} else {
		stateStr = "Idle"
	}

	project := cfg.WorkingDir
	if project == "" {
		project = "Not linked"
	}

	text := fmt.Sprintf(
		"GSD Telegram Bot v1.0\n\nProject: %s\nStatus: %s\n\nCommands:\n/new — Start a new Claude session\n/stop — Stop the current query\n/status — Show session info\n/resume — Restore a previous session",
		project,
		stateStr,
	)

	_, err := b.SendMessage(chatID, text, nil)
	return err
}

// HandleNew handles the /new command (CMD-02).
// Creates a fresh Claude session for the channel. If a session is running,
// it is stopped first. The previous session ID is cleared so the next query
// starts a new Claude session.
func HandleNew(b *gotgbot.Bot, ctx *ext.Context, store *session.SessionStore, persist *session.PersistenceManager, cfg *config.Config) error {
	chatID := ctx.EffectiveChat.Id
	sess := store.GetOrCreate(chatID, cfg.WorkingDir)

	if sess.IsRunning() {
		sess.Stop()
		// Brief wait so the worker goroutine has a chance to transition to idle.
		time.Sleep(100 * time.Millisecond)
	}

	// Clear the session ID so the next query starts a brand-new Claude session.
	// The previous session ID was already persisted by the OnQueryComplete callback
	// when the last query completed, so it will appear in /resume.
	sess.SetSessionID("")

	_, err := b.SendMessage(chatID, "New session started. Previous session saved for /resume.", nil)
	return err
}

// HandleStop handles the /stop command (CMD-03 / SESS-05).
// Stops the currently running Claude query. If no query is running, replies accordingly.
func HandleStop(b *gotgbot.Bot, ctx *ext.Context, store *session.SessionStore) error {
	chatID := ctx.EffectiveChat.Id

	sess, ok := store.Get(chatID)
	if !ok || !sess.IsRunning() {
		_, err := b.SendMessage(chatID, "No query running.", nil)
		return err
	}

	sess.Stop()
	_, err := b.SendMessage(chatID, "Query stopped.", nil)
	return err
}

// HandleStatus handles the /status command (CMD-04).
// Displays a status dashboard: session state, query state, token usage, context percent, project path.
func HandleStatus(b *gotgbot.Bot, ctx *ext.Context, store *session.SessionStore, cfg *config.Config) error {
	chatID := ctx.EffectiveChat.Id

	sess, ok := store.Get(chatID)
	var text string
	if !ok {
		// No session ever created for this channel.
		text = buildStatusText(nil, cfg.WorkingDir)
	} else {
		text = buildStatusText(sess, cfg.WorkingDir)
	}

	_, err := b.SendMessage(chatID, text, nil)
	return err
}

// buildStatusText builds the formatted status dashboard string.
// sess may be nil if no session has been created for the channel.
//
// Output format (CONTEXT.md locked):
//
//	Session: Active (abc12345...) | Session: None
//	Query: Running (12s) | Query: Idle
//	  [tool status if running]
//
//	Tokens: in=1234 out=567 cache_read=890 cache_create=12
//	Context: 42%
//
//	Project: /path/to/project
func buildStatusText(sess *session.Session, workingDir string) string {
	var sb strings.Builder

	// --- Session line ---
	if sess == nil || sess.SessionID() == "" {
		sb.WriteString("Session: None")
	} else {
		id := sess.SessionID()
		short := id
		if len(id) > 8 {
			short = id[:8]
		}
		sb.WriteString(fmt.Sprintf("Session: Active (%s...)", short))
	}
	sb.WriteString("\n")

	// --- Query line ---
	if sess != nil && sess.IsRunning() {
		elapsed := ""
		if qs := sess.QueryStarted(); qs != nil {
			secs := int(time.Since(*qs).Seconds())
			elapsed = fmt.Sprintf(" (%ds)", secs)
		}
		sb.WriteString(fmt.Sprintf("Query: Running%s", elapsed))

		// Tool status (if a tool is currently executing).
		if tool := sess.CurrentTool(); tool != "" {
			sb.WriteString("\n  ")
			sb.WriteString(tool)
		}
	} else {
		sb.WriteString("Query: Idle")
	}
	sb.WriteString("\n")

	// --- Token usage (only if available) ---
	if sess != nil {
		if usage := sess.LastUsage(); usage != nil {
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf(
				"Tokens: in=%d out=%d cache_read=%d cache_create=%d",
				usage.InputTokens,
				usage.OutputTokens,
				usage.CacheReadInputTokens,
				usage.CacheCreationInputTokens,
			))
			sb.WriteString("\n")
		}

		// --- Context percent (only if available) ---
		if pct := sess.ContextPercent(); pct != nil {
			sb.WriteString(fmt.Sprintf("Context: %d%%", *pct))
			sb.WriteString("\n")
		}
	}

	// --- Project path ---
	sb.WriteString("\n")
	project := workingDir
	if project == "" {
		project = "Not linked"
	}
	sb.WriteString(fmt.Sprintf("Project: %s", project))

	return sb.String()
}

// HandleResume handles the /resume command (CMD-05).
// Lists saved sessions as an inline keyboard. Tapping a button restores that session.
func HandleResume(b *gotgbot.Bot, ctx *ext.Context, persist *session.PersistenceManager) error {
	chatID := ctx.EffectiveChat.Id

	sessions, err := persist.LoadForChannel(chatID)
	if err != nil {
		return fmt.Errorf("loading saved sessions: %w", err)
	}

	if len(sessions) == 0 {
		_, err := b.SendMessage(chatID, "No saved sessions found.", nil)
		return err
	}

	// Build one inline keyboard button per saved session.
	// Button label: "<timestamp> - <title>" (title trimmed to ~30 chars).
	// Callback data: "resume:<session_id>" (~43 chars — well under 64-byte limit).
	rows := make([][]gotgbot.InlineKeyboardButton, 0, len(sessions))
	for _, s := range sessions {
		label := formatSessionLabel(s)
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{
				Text:         label,
				CallbackData: "resume:" + s.SessionID,
			},
		})
	}

	keyboard := gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
	_, err = b.SendMessage(chatID, "Select a session to resume:", &gotgbot.SendMessageOpts{
		ReplyMarkup: keyboard,
	})
	return err
}

// formatSessionLabel builds the display label for a session button.
// Format: "<saved_at> - <title>" with title capped at ButtonLabelMaxLength chars.
func formatSessionLabel(s session.SavedSession) string {
	// Parse and reformat the ISO 8601 timestamp to something readable.
	ts := s.SavedAt
	if t, err := time.Parse(time.RFC3339, s.SavedAt); err == nil {
		ts = t.UTC().Format("2006-01-02 15:04")
	}

	title := s.Title
	maxTitle := config.ButtonLabelMaxLength
	if len(title) > maxTitle {
		title = title[:maxTitle]
	}
	if title == "" {
		title = "(no title)"
	}

	return fmt.Sprintf("%s - %s", ts, title)
}
