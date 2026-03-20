package formatting

import (
	"fmt"
	"strings"
)

// toolEmojiMap maps tool name substrings to display emojis.
// Keys are checked using strings.Contains on the tool name.
var toolEmojiMap = map[string]string{
	"Read":      "📖",
	"Write":     "📝",
	"Edit":      "✏️",
	"Bash":      "▶️",
	"Glob":      "🔍",
	"Grep":      "🔎",
	"WebSearch": "🔍",
	"WebFetch":  "🌐",
	"Task":      "🎯",
	"TodoWrite": "📋",
	"mcp__":    "🔧",
}

// toolEmojiOrder defines lookup order to ensure specific tools match before
// generic ones (e.g., "WebSearch" before "Search", "Grep" before "Glob").
var toolEmojiOrder = []string{
	"WebSearch",
	"WebFetch",
	"TodoWrite",
	"Read",
	"Write",
	"Edit",
	"Bash",
	"Grep",
	"Glob",
	"Task",
	"mcp__",
}

// FormatToolStatus formats a tool use event as a short human-readable status
// string with an emoji indicator. The output matches the TypeScript
// formatToolStatus behavior, adapted for MarkdownV2 (plain text, no HTML tags).
//
// toolName is the name of the Claude tool (e.g., "Read", "Bash", "mcp__github__list_repos").
// toolInput is the tool's input map (e.g., {"file_path": "/path/to/file"}).
func FormatToolStatus(toolName string, toolInput map[string]any) string {
	// Find matching emoji using ordered lookup
	emoji := "🔧"
	for _, key := range toolEmojiOrder {
		if strings.Contains(toolName, key) {
			emoji = toolEmojiMap[key]
			break
		}
	}

	// Per-tool formatting
	switch {
	case toolName == "Read":
		filePath := stringField(toolInput, "file_path", "file")
		shortPath := shortenPath(filePath)
		return fmt.Sprintf("%s Reading %s", emoji, shortPath)

	case toolName == "Write":
		filePath := stringField(toolInput, "file_path", "file")
		return fmt.Sprintf("%s Writing %s", emoji, shortenPath(filePath))

	case toolName == "Edit":
		filePath := stringField(toolInput, "file_path", "file")
		return fmt.Sprintf("%s Editing %s", emoji, shortenPath(filePath))

	case toolName == "Bash":
		desc := stringField(toolInput, "description", "")
		if desc != "" {
			return fmt.Sprintf("%s %s", emoji, truncate(desc, 60))
		}
		cmd := stringField(toolInput, "command", "")
		return fmt.Sprintf("%s %s", emoji, truncate(cmd, 50))

	case toolName == "Grep":
		pattern := stringField(toolInput, "pattern", "")
		path := stringField(toolInput, "path", "")
		if path != "" {
			return fmt.Sprintf("%s Searching %s in %s", emoji,
				truncate(pattern, 30), shortenPath(path))
		}
		return fmt.Sprintf("%s Searching %s", emoji, truncate(pattern, 40))

	case toolName == "Glob":
		pattern := stringField(toolInput, "pattern", "")
		return fmt.Sprintf("%s Finding %s", emoji, truncate(pattern, 50))

	case toolName == "WebSearch":
		query := stringField(toolInput, "query", "")
		return fmt.Sprintf("%s Searching: %s", emoji, truncate(query, 50))

	case toolName == "WebFetch":
		url := stringField(toolInput, "url", "")
		return fmt.Sprintf("%s Fetching %s", emoji, truncate(url, 50))

	case toolName == "Task":
		desc := stringField(toolInput, "description", "")
		if desc != "" {
			return fmt.Sprintf("%s Agent: %s", emoji, truncate(desc, 60))
		}
		return fmt.Sprintf("%s Running agent...", emoji)

	case toolName == "TodoWrite":
		return fmt.Sprintf("%s Updating todos", emoji)

	case strings.HasPrefix(toolName, "mcp__"):
		return formatMCPTool(toolName, toolInput)
	}

	// Default fallback
	return fmt.Sprintf("%s %s", emoji, toolName)
}

// formatMCPTool formats an MCP tool name like "mcp__github__list_repos" into a
// human-readable status: "🔧 github list repos: <summary>"
func formatMCPTool(toolName string, toolInput map[string]any) string {
	parts := strings.Split(toolName, "__")
	if len(parts) >= 3 {
		server := parts[1]
		action := parts[2]
		// Remove redundant server prefix from action
		if strings.HasPrefix(action, server+"_") {
			action = action[len(server)+1:]
		}
		action = strings.ReplaceAll(action, "_", " ")

		// Try to get a meaningful summary from common input fields
		summary := ""
		for _, field := range []string{"title", "query", "content", "text", "id"} {
			if v := stringField(toolInput, field, ""); v != "" {
				summary = truncate(v, 40)
				break
			}
		}

		if summary != "" {
			return fmt.Sprintf("🔧 %s %s: %s", server, action, summary)
		}
		return fmt.Sprintf("🔧 %s: %s", server, action)
	}
	return fmt.Sprintf("🔧 %s", toolName)
}

// shortenPath returns the last 2 path components of a file path for display.
// Works with both forward slashes (Unix) and backslashes (Windows).
func shortenPath(path string) string {
	if path == "" {
		return "file"
	}
	// Normalize separators
	normalized := strings.ReplaceAll(path, `\`, "/")
	parts := strings.Split(normalized, "/")
	// Filter empty parts (from leading slash)
	nonEmpty := parts[:0]
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return path
	}
	if len(nonEmpty) == 1 {
		return nonEmpty[0]
	}
	return nonEmpty[len(nonEmpty)-2] + "/" + nonEmpty[len(nonEmpty)-1]
}

// truncate returns text truncated to maxLen characters, appending "..." if
// truncated. Newlines are collapsed to spaces for single-line display.
func truncate(text string, maxLen int) string {
	if text == "" {
		return ""
	}
	// Clean up newlines for single-line display
	cleaned := strings.ReplaceAll(text, "\n", " ")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) <= maxLen {
		return cleaned
	}
	return cleaned[:maxLen] + "..."
}

// stringField extracts a string value from a map[string]any, returning
// defaultVal if the key is absent or not a string.
func stringField(m map[string]any, key, defaultVal string) string {
	if m == nil {
		return defaultVal
	}
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}
