package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/rs/zerolog/log"

	"github.com/user/gsd-tele-go/internal/audit"
	"github.com/user/gsd-tele-go/internal/claude"
	"github.com/user/gsd-tele-go/internal/config"
	"github.com/user/gsd-tele-go/internal/project"
	"github.com/user/gsd-tele-go/internal/security"
	"github.com/user/gsd-tele-go/internal/session"
)

// AwaitingPathState tracks which channels are waiting for the user to supply a
// project directory path. Thread-safe.
type AwaitingPathState struct {
	mu       sync.Mutex
	channels map[int64]bool
}

// NewAwaitingPathState creates an empty AwaitingPathState.
func NewAwaitingPathState() *AwaitingPathState {
	return &AwaitingPathState{channels: make(map[int64]bool)}
}

// IsAwaiting reports whether chatID is currently waiting for a path reply.
func (a *AwaitingPathState) IsAwaiting(chatID int64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.channels[chatID]
}

// Set marks chatID as waiting for a path reply.
func (a *AwaitingPathState) Set(chatID int64) {
	a.mu.Lock()
	a.channels[chatID] = true
	a.mu.Unlock()
}

// Clear removes the awaiting-path mark for chatID.
func (a *AwaitingPathState) Clear(chatID int64) {
	a.mu.Lock()
	delete(a.channels, chatID)
	a.mu.Unlock()
}

// HandleText processes a text message, routing it to the channel's Claude session.
//
// The wg parameter is the bot's WaitGroup.  HandleText calls wg.Add(1) when starting
// a new session worker goroutine, ensuring the Bot tracks all active workers for
// graceful shutdown.
//
// mappings and awaitingPath support per-channel project routing. If the channel has
// no project mapping, the user is prompted to supply a directory path.
func HandleText(
	tgBot *gotgbot.Bot,
	ctx *ext.Context,
	store *session.SessionStore,
	cfg *config.Config,
	auditLog *audit.Logger,
	persist *session.PersistenceManager,
	wg *sync.WaitGroup,
	mappings *project.MappingStore,
	awaitingPath *AwaitingPathState,
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

	// --- Mapping check ---
	// 1. If awaiting path input, handle it (returns after saving or prompting again).
	if awaitingPath.IsAwaiting(chatID) {
		return handlePathInput(tgBot, chatID, text, mappings, awaitingPath, cfg)
	}

	// 2. Check if channel has a project mapping.
	mapping, hasMapped := mappings.Get(chatID)
	if !hasMapped {
		awaitingPath.Set(chatID)
		_, err := tgBot.SendMessage(chatID,
			"This channel has no linked project. Reply with the full directory path to link it.", nil)
		return err
	}

	// --- Interrupt handling ---
	// A message prefixed with "!" interrupts the running query and uses the
	// stripped text as the new message.
	sess := store.GetOrCreate(chatID, mapping.Path)
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
	// We use context.Background() so the worker is not tied to the per-update goroutine.
	// Shutdown is handled via the session's stop channel and WaitGroup.
	capturedText := text
	capturedChatID := chatID
	capturedUserID := userID
	capturedMapping := mapping

	workerCfg := session.WorkerConfig{
		AllowedPaths: []string{mapping.Path},
		SafetyPrompt: config.BuildSafetyPrompt([]string{mapping.Path}),
		FilteredEnv:  config.FilteredEnv(),
		OnQueryComplete: func(sessionID string) {
			// CRITICAL: Use mapping.Path as WorkingDir for per-project session persistence.
			// The PersistenceManager trims per WorkingDir (maxPerProject=5), so setting
			// WorkingDir=mapping.Path ensures each project keeps its own history.
			title := capturedText
			if len(title) > 50 {
				title = title[:50]
			}
			saved := session.SavedSession{
				SessionID:  sessionID,
				SavedAt:    time.Now().UTC().Format(time.RFC3339),
				WorkingDir: capturedMapping.Path,
				Title:      title,
				ChannelID:  capturedChatID,
			}
			if err := persist.Save(saved); err != nil {
				log.Warn().Err(err).Msg("Failed to persist session")
			}
		},
	}

	// Start the worker goroutine exactly once per session using the workerStarted flag.
	shouldStartWorker := !sess.WorkerStarted()
	if shouldStartWorker {
		sess.SetWorkerStarted()
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

// handlePathInput handles a text message from a channel that is awaiting a project
// directory path. Validates the path and saves the mapping on success.
func handlePathInput(tgBot *gotgbot.Bot, chatID int64, text string, mappings *project.MappingStore, awaitingPath *AwaitingPathState, cfg *config.Config) error {
	text = strings.TrimSpace(text)

	// Validate path is under ALLOWED_PATHS.
	if !security.ValidatePath(text, cfg.AllowedPaths) {
		_, err := tgBot.SendMessage(chatID,
			fmt.Sprintf("Path not allowed. Must be under: %s\nTry again:", strings.Join(cfg.AllowedPaths, ", ")), nil)
		return err
	}

	// Validate path exists on filesystem.
	info, statErr := os.Stat(text)
	if statErr != nil || !info.IsDir() {
		_, err := tgBot.SendMessage(chatID, "Directory not found. Try again with a valid path:", nil)
		return err
	}

	// Save mapping.
	m := project.ProjectMapping{
		Path:     text,
		Name:     filepath.Base(text),
		LinkedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := mappings.Set(chatID, m); err != nil {
		_, _ = tgBot.SendMessage(chatID, "Failed to save mapping: "+err.Error(), nil)
		return err
	}

	awaitingPath.Clear(chatID)
	_, err := tgBot.SendMessage(chatID,
		fmt.Sprintf("Linked to %s. Send a message to start working.", text), nil)
	return err
}
