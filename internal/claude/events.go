// Package claude provides Claude CLI subprocess management with NDJSON streaming.
package claude

import (
	"encoding/json"
	"regexp"
)

// ClaudeEvent represents a single NDJSON event emitted by the Claude CLI
// when using --output-format stream-json.
type ClaudeEvent struct {
	// Type is the event type: "assistant", "result", or "system".
	Type string `json:"type"`

	// SessionID is the Claude session identifier (present on most events).
	SessionID string `json:"session_id"`

	// Message is populated for "assistant" type events.
	Message *AssistantMsg `json:"message"`

	// Result holds the final response text on "result" type events.
	Result string `json:"result"`

	// IsError indicates that the result event represents an error.
	IsError bool `json:"is_error"`

	// Subtype is the event subtype (e.g. "error" on error result events).
	Subtype string `json:"subtype"`

	// Error holds the error message for error events.
	Error string `json:"error"`

	// Usage holds token usage statistics (present on "result" events).
	Usage *UsageData `json:"usage"`

	// ModelUsage maps model name to per-model usage data (present on "result" events).
	// The value is kept as raw JSON to allow flexible parsing of the modelUsage field,
	// which may vary across CLI versions.
	ModelUsage map[string]json.RawMessage `json:"modelUsage"`
}

// ContextPercent calculates the context window utilisation percentage from ModelUsage.
// Returns nil if ModelUsage is empty or contextWindow is 0.
func (e *ClaudeEvent) ContextPercent() *int {
	if len(e.ModelUsage) == 0 {
		return nil
	}

	// Use the first model entry.
	for _, raw := range e.ModelUsage {
		var entry ModelUsageEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			return nil
		}
		if entry.ContextWindow == 0 {
			return nil
		}
		pct := (entry.InputTokens + entry.OutputTokens) * 100 / entry.ContextWindow
		return &pct
	}
	return nil
}

// AssistantMsg is the message payload inside an "assistant" event.
type AssistantMsg struct {
	// ID is the message identifier (used for deduplication across partial updates).
	ID string `json:"id"`

	// Content holds the ordered sequence of content blocks.
	Content []ContentBlock `json:"content"`
}

// ContentBlock is one block inside an assistant message.
// The Type field determines which other fields are populated.
type ContentBlock struct {
	// Type is "text", "thinking", "tool_use", or "tool_result".
	Type string `json:"type"`

	// Text is the text content (populated when Type == "text").
	Text string `json:"text"`

	// Thinking is the reasoning text (populated when Type == "thinking").
	Thinking string `json:"thinking"`

	// ID is the unique identifier for a tool_use block.
	ID string `json:"id"`

	// Name is the tool name (e.g. "Read", "Write", "Bash") for tool_use blocks.
	Name string `json:"name"`

	// Input holds the tool call arguments for tool_use blocks.
	Input map[string]any `json:"input"`
}

// UsageData holds token usage statistics from the Claude API.
type UsageData struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// ModelUsageEntry holds per-model context window usage statistics.
// These are the camelCase fields from the Claude CLI's modelUsage map.
type ModelUsageEntry struct {
	InputTokens              int `json:"inputTokens"`
	OutputTokens             int `json:"outputTokens"`
	CacheReadInputTokens     int `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int `json:"cacheCreationInputTokens"`
	ContextWindow            int `json:"contextWindow"`
}

// contextLimitPatterns is the set of compiled patterns for detecting context limit errors.
// Compiled at package init time to avoid repeated compilation.
var contextLimitPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)input length and max_tokens exceed context limit`),
	regexp.MustCompile(`(?i)exceed context limit`),
	regexp.MustCompile(`(?i)context limit.*exceeded`),
	regexp.MustCompile(`(?i)prompt.*too.*long`),
	regexp.MustCompile(`(?i)conversation is too long`),
}

// isContextLimitError reports whether text matches any known context limit error pattern.
func isContextLimitError(text string) bool {
	for _, re := range contextLimitPatterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// BuildArgs constructs the Claude CLI argument slice for a streaming session.
// The prompt is NOT included — callers must pipe it via stdin.
//
// Arg construction mirrors the TypeScript session.ts sendMessageStreaming method.
func BuildArgs(sessionID string, allowedPaths []string, model, systemPrompt string) []string {
	args := []string{
		"-p",
		"--verbose",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--dangerously-skip-permissions",
	}

	// Additional directories Claude is permitted to access.
	if len(allowedPaths) > 0 {
		args = append(args, "--add-dir")
		args = append(args, allowedPaths...)
	}

	// Resume an existing session.
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}

	// Optional model override.
	if model != "" {
		args = append(args, "--model", model)
	}

	// Optional system prompt appendage.
	if systemPrompt != "" {
		args = append(args, "--append-system-prompt", systemPrompt)
	}

	return args
}
