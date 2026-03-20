package formatting

import (
	"strings"
	"testing"
)

// TestFormatToolStatusRead verifies Read tool formatting.
func TestFormatToolStatusRead(t *testing.T) {
	result := FormatToolStatus("Read", map[string]any{
		"file_path": "/home/user/projects/src/main.go",
	})
	if !strings.Contains(result, "📖") {
		t.Errorf("FormatToolStatus Read: missing 📖 emoji, got: %q", result)
	}
	if !strings.Contains(result, "src/main.go") {
		t.Errorf("FormatToolStatus Read: missing shortened path 'src/main.go', got: %q", result)
	}
}

// TestFormatToolStatusWrite verifies Write tool formatting.
func TestFormatToolStatusWrite(t *testing.T) {
	result := FormatToolStatus("Write", map[string]any{
		"file_path": "/tmp/output/result.json",
	})
	if !strings.Contains(result, "📝") {
		t.Errorf("FormatToolStatus Write: missing 📝 emoji, got: %q", result)
	}
	if !strings.Contains(result, "output/result.json") {
		t.Errorf("FormatToolStatus Write: missing shortened path, got: %q", result)
	}
}

// TestFormatToolStatusEdit verifies Edit tool formatting.
func TestFormatToolStatusEdit(t *testing.T) {
	result := FormatToolStatus("Edit", map[string]any{
		"file_path": "/projects/app/config.yaml",
	})
	if !strings.Contains(result, "✏️") {
		t.Errorf("FormatToolStatus Edit: missing ✏️ emoji, got: %q", result)
	}
	if !strings.Contains(result, "app/config.yaml") {
		t.Errorf("FormatToolStatus Edit: missing shortened path, got: %q", result)
	}
}

// TestFormatToolStatusBash verifies Bash tool uses description when available.
func TestFormatToolStatusBash(t *testing.T) {
	result := FormatToolStatus("Bash", map[string]any{
		"command":     "ls -la",
		"description": "List files",
	})
	if !strings.Contains(result, "▶️") {
		t.Errorf("FormatToolStatus Bash: missing ▶️ emoji, got: %q", result)
	}
	if !strings.Contains(result, "List files") {
		t.Errorf("FormatToolStatus Bash: should show description, got: %q", result)
	}
}

// TestFormatToolStatusBashNoDesc verifies Bash truncates command when no description.
func TestFormatToolStatusBashNoDesc(t *testing.T) {
	longCmd := "some very long command that goes on and on and on and on and should be truncated"
	result := FormatToolStatus("Bash", map[string]any{
		"command": longCmd,
	})
	if !strings.Contains(result, "▶️") {
		t.Errorf("FormatToolStatus Bash no-desc: missing ▶️ emoji, got: %q", result)
	}
	if strings.Contains(result, longCmd) {
		t.Errorf("FormatToolStatus Bash no-desc: should truncate long command, got: %q", result)
	}
	if !strings.Contains(result, "...") {
		t.Errorf("FormatToolStatus Bash no-desc: truncated text should have '...', got: %q", result)
	}
}

// TestFormatToolStatusGrep verifies Grep tool formatting.
func TestFormatToolStatusGrep(t *testing.T) {
	result := FormatToolStatus("Grep", map[string]any{
		"pattern": "TODO",
		"path":    "/src",
	})
	if !strings.Contains(result, "🔎") {
		t.Errorf("FormatToolStatus Grep: missing 🔎 emoji, got: %q", result)
	}
	if !strings.Contains(result, "TODO") {
		t.Errorf("FormatToolStatus Grep: missing pattern 'TODO', got: %q", result)
	}
}

// TestFormatToolStatusGlob verifies Glob tool formatting.
func TestFormatToolStatusGlob(t *testing.T) {
	result := FormatToolStatus("Glob", map[string]any{
		"pattern": "**/*.go",
	})
	if !strings.Contains(result, "🔍") {
		t.Errorf("FormatToolStatus Glob: missing 🔍 emoji, got: %q", result)
	}
	if !strings.Contains(result, "**/*.go") {
		t.Errorf("FormatToolStatus Glob: missing pattern, got: %q", result)
	}
}

// TestFormatToolStatusWebSearch verifies WebSearch tool formatting.
func TestFormatToolStatusWebSearch(t *testing.T) {
	result := FormatToolStatus("WebSearch", map[string]any{
		"query": "Go Telegram bot library",
	})
	if !strings.Contains(result, "🔍") {
		t.Errorf("FormatToolStatus WebSearch: missing 🔍 emoji, got: %q", result)
	}
	if !strings.Contains(result, "Go Telegram bot library") {
		t.Errorf("FormatToolStatus WebSearch: missing query, got: %q", result)
	}
}

// TestFormatToolStatusWebFetch verifies WebFetch tool formatting.
func TestFormatToolStatusWebFetch(t *testing.T) {
	result := FormatToolStatus("WebFetch", map[string]any{
		"url": "https://example.com/api",
	})
	if !strings.Contains(result, "🌐") {
		t.Errorf("FormatToolStatus WebFetch: missing 🌐 emoji, got: %q", result)
	}
	if !strings.Contains(result, "https://example.com/api") {
		t.Errorf("FormatToolStatus WebFetch: missing url, got: %q", result)
	}
}

// TestFormatToolStatusTask verifies Task (agent) tool formatting.
func TestFormatToolStatusTask(t *testing.T) {
	result := FormatToolStatus("Task", map[string]any{
		"description": "Analyze codebase",
	})
	if !strings.Contains(result, "🎯") {
		t.Errorf("FormatToolStatus Task: missing 🎯 emoji, got: %q", result)
	}
	if !strings.Contains(result, "Analyze codebase") {
		t.Errorf("FormatToolStatus Task: missing description, got: %q", result)
	}
}

// TestFormatToolStatusMCP verifies MCP tool formatting shows server name.
func TestFormatToolStatusMCP(t *testing.T) {
	result := FormatToolStatus("mcp__github__list_repos", map[string]any{})
	if !strings.Contains(result, "🔧") {
		t.Errorf("FormatToolStatus MCP: missing 🔧 emoji, got: %q", result)
	}
	if !strings.Contains(result, "github") {
		t.Errorf("FormatToolStatus MCP: missing server name 'github', got: %q", result)
	}
}

// TestFormatToolStatusMCPWithSummary verifies MCP tool shows summary field.
func TestFormatToolStatusMCPWithSummary(t *testing.T) {
	result := FormatToolStatus("mcp__github__create_issue", map[string]any{
		"title": "Fix the bug",
	})
	if !strings.Contains(result, "🔧") {
		t.Errorf("FormatToolStatus MCP with summary: missing 🔧 emoji, got: %q", result)
	}
	if !strings.Contains(result, "Fix the bug") {
		t.Errorf("FormatToolStatus MCP with summary: missing title in output, got: %q", result)
	}
}

// TestFormatToolStatusUnknown verifies unknown tools get default formatting.
func TestFormatToolStatusUnknown(t *testing.T) {
	result := FormatToolStatus("CustomTool", map[string]any{})
	if !strings.Contains(result, "🔧") {
		t.Errorf("FormatToolStatus unknown: missing default 🔧 emoji, got: %q", result)
	}
	if !strings.Contains(result, "CustomTool") {
		t.Errorf("FormatToolStatus unknown: missing tool name, got: %q", result)
	}
}

// TestShortenPath verifies path shortening to last 2 components.
func TestShortenPath(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/home/user/projects/src/main.go", "src/main.go"},
		{"/tmp/file.txt", "tmp/file.txt"},
		{"file.txt", "file.txt"},
		{"", "file"},
		{"/a/b/c/d/e/f", "e/f"},
		{`C:\Users\user\Documents\file.txt`, "Documents/file.txt"},
	}
	for _, c := range cases {
		got := shortenPath(c.input)
		if got != c.want {
			t.Errorf("shortenPath(%q) = %q; want %q", c.input, got, c.want)
		}
	}
}

// TestTruncate verifies truncation at maxLen with ellipsis.
func TestTruncate(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := truncate(long, 50)
	want := strings.Repeat("a", 50) + "..."
	if got != want {
		t.Errorf("truncate(100-char, 50) = %q; want %q", got, want)
	}

	// Short text unchanged
	short := "hello"
	got = truncate(short, 50)
	if got != short {
		t.Errorf("truncate(%q, 50) = %q; want %q", short, got, short)
	}

	// Empty string
	got = truncate("", 50)
	if got != "" {
		t.Errorf("truncate('', 50) = %q; want ''", got)
	}
}

// TestTruncateNewlines verifies newlines are collapsed in truncated output.
func TestTruncateNewlines(t *testing.T) {
	input := "line one\nline two\nline three"
	got := truncate(input, 100)
	if strings.Contains(got, "\n") {
		t.Errorf("truncate should collapse newlines, got: %q", got)
	}
}

// TestFormatToolStatusNilInput verifies nil toolInput doesn't panic.
func TestFormatToolStatusNilInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("FormatToolStatus panicked with nil input: %v", r)
		}
	}()
	result := FormatToolStatus("Read", nil)
	if result == "" {
		t.Error("FormatToolStatus with nil input should return non-empty string")
	}
}
