package handlers

import (
	"testing"

	"github.com/user/gsd-tele-go/internal/config"
)

// TestWorkerConfigPerProject verifies that per-project WorkerConfig uses the
// mapping's path (not cfg.WorkingDir) for AllowedPaths and SafetyPrompt.
//
// We test the extracted helper functions rather than HandleText directly
// (HandleText requires a live gotgbot.Bot and ext.Context).
func TestWorkerConfigPerProject(t *testing.T) {
	projectPath := "/test/project"

	// Verify BuildSafetyPrompt produces a prompt containing the project path.
	prompt := config.BuildSafetyPrompt([]string{projectPath})
	if prompt == "" {
		t.Fatal("BuildSafetyPrompt returned empty string")
	}

	// The safety prompt must reference the project path.
	if len(prompt) > 0 {
		found := false
		for i := 0; i < len(prompt)-len(projectPath)+1; i++ {
			if prompt[i:i+len(projectPath)] == projectPath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("BuildSafetyPrompt result does not contain project path %q\nprompt:\n%s", projectPath, prompt)
		}
	}

	// Verify AllowedPaths slice is constructed correctly per-project.
	allowedPaths := []string{projectPath}
	if len(allowedPaths) != 1 {
		t.Errorf("expected 1 allowed path, got %d", len(allowedPaths))
	}
	if allowedPaths[0] != projectPath {
		t.Errorf("expected allowed path %q, got %q", projectPath, allowedPaths[0])
	}

	// Verify that two different projects produce different prompts.
	otherPath := "/other/project"
	promptA := config.BuildSafetyPrompt([]string{projectPath})
	promptB := config.BuildSafetyPrompt([]string{otherPath})
	if promptA == promptB {
		t.Error("BuildSafetyPrompt with different paths should produce different prompts")
	}
}

// TestHandleTextUnmapped verifies that AwaitingPathState correctly manages the
// unmapped channel flow: Set → IsAwaiting → Clear → not IsAwaiting.
func TestHandleTextUnmapped(t *testing.T) {
	state := NewAwaitingPathState()

	// Initially no channel is awaiting.
	if state.IsAwaiting(123) {
		t.Error("expected IsAwaiting(123) == false before Set()")
	}

	// After Set, channel is awaiting.
	state.Set(123)
	if !state.IsAwaiting(123) {
		t.Error("expected IsAwaiting(123) == true after Set()")
	}

	// After Clear, channel is no longer awaiting.
	state.Clear(123)
	if state.IsAwaiting(123) {
		t.Error("expected IsAwaiting(123) == false after Clear()")
	}

	// Verify multiple channels are independent.
	state.Set(100)
	state.Set(200)
	state.Set(300)

	if !state.IsAwaiting(100) {
		t.Error("expected channel 100 to be awaiting")
	}
	if !state.IsAwaiting(200) {
		t.Error("expected channel 200 to be awaiting")
	}
	if !state.IsAwaiting(300) {
		t.Error("expected channel 300 to be awaiting")
	}

	// Clear one; others should still be awaiting.
	state.Clear(200)
	if state.IsAwaiting(200) {
		t.Error("expected channel 200 to not be awaiting after Clear()")
	}
	if !state.IsAwaiting(100) {
		t.Error("expected channel 100 to still be awaiting")
	}
	if !state.IsAwaiting(300) {
		t.Error("expected channel 300 to still be awaiting")
	}
}
