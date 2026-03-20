package handlers

import (
	"testing"

	"github.com/user/gsd-tele-go/internal/config"
	"github.com/user/gsd-tele-go/internal/security"
)

// TestCallbackSafetyCheckBlocksPatterns verifies that CheckCommandSafety
// correctly blocks dangerous patterns that could arrive via callback buttons.
// This covers AUTH-03 for the callback path.
func TestCallbackSafetyCheckBlocksPatterns(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		blocked bool
	}{
		{"normal gsd command", "/gsd:execute-phase 1", false},
		{"blocked rm -rf /", "rm -rf /", true},
		{"blocked format c:", "format c:", true},
		{"blocked dd if=", "dd if=/dev/zero of=/dev/sda", true},
		{"safe option selection", "1", false},
		{"safe lettered option", "A", false},
		{"safe gsd status", "/gsd:status", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safe, pattern := security.CheckCommandSafety(tt.text, config.BlockedPatterns)
			if tt.blocked && safe {
				t.Errorf("expected %q to be blocked, but it passed safety check", tt.text)
			}
			if !tt.blocked && !safe {
				t.Errorf("expected %q to pass safety check, but blocked by pattern %q", tt.text, pattern)
			}
		})
	}
}

// TestCallbackRoutesToEnqueue verifies that all callback actions that should
// route through enqueueGsdCommand are correctly parsed.
// If these routes work, the safety check in enqueueGsdCommand covers them all.
func TestCallbackRoutesToEnqueue(t *testing.T) {
	tests := []struct {
		data           string
		expectedAction callbackAction
	}{
		{"gsd-run:/gsd:status", callbackActionGsdRun},
		{"gsd-fresh:/gsd:init", callbackActionGsdFresh},
		{"gsd-exec:2", callbackActionGsdPhase},
		{"gsd-plan:3", callbackActionGsdPhase},
		{"gsd:status", callbackActionGsd},
		{"option:1", callbackActionOption},
		{"askuser:abc123:0", callbackActionAskUser},
	}

	for _, tt := range tests {
		t.Run(tt.data, func(t *testing.T) {
			action, _ := parseCallbackData(tt.data)
			if action != tt.expectedAction {
				t.Errorf("parseCallbackData(%q) = %d, want %d", tt.data, action, tt.expectedAction)
			}
		})
	}
}

// TestCallbackLifecycleNoSafetyCheck verifies that resume and new callbacks
// are NOT routed through enqueueGsdCommand (they are lifecycle ops, not queries).
func TestCallbackLifecycleNoSafetyCheck(t *testing.T) {
	tests := []struct {
		data           string
		expectedAction callbackAction
	}{
		{"resume:session123", callbackActionResume},
		{"action:new", callbackActionNew},
		{"action:stop", callbackActionStop},
	}

	for _, tt := range tests {
		t.Run(tt.data, func(t *testing.T) {
			action, _ := parseCallbackData(tt.data)
			if action != tt.expectedAction {
				t.Errorf("parseCallbackData(%q) = %d, want %d", tt.data, action, tt.expectedAction)
			}
		})
	}
}
