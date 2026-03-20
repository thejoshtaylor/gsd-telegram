// Package audit provides a goroutine-safe append-only audit logger.
// Each log entry is written as a single JSON line (NDJSON format).
package audit

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Event represents a single audit log entry.
type Event struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username,omitempty"`
	ChannelID int64  `json:"channel_id"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

// NewEvent creates an Event with Timestamp set to the current UTC time in RFC3339 format.
func NewEvent(action string, userID int64, channelID int64) Event {
	return Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Action:    action,
		UserID:    userID,
		ChannelID: channelID,
	}
}

// Logger is a goroutine-safe append-only audit logger that writes JSON lines to a file.
type Logger struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
}

// New opens (or creates) the file at path for append-only writing and returns a Logger.
// The file is opened with O_APPEND|O_CREATE|O_WRONLY flags.
func New(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &Logger{
		file:    f,
		encoder: json.NewEncoder(f),
	}, nil
}

// Log writes event as a JSON line to the log file.
// It is safe to call concurrently from multiple goroutines.
func (l *Logger) Log(event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.encoder.Encode(event)
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
