package config

import (
	"strings"
	"testing"
)

// TestLoadNodeConfig verifies that LoadNodeConfig populates all fields from env vars.
func TestLoadNodeConfig(t *testing.T) {
	t.Setenv("SERVER_URL", "wss://example.com/ws")
	t.Setenv("SERVER_TOKEN", "secret")
	t.Setenv("HEARTBEAT_INTERVAL_SECS", "45")

	cfg, err := LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig() returned error: %v", err)
	}

	if cfg.ServerURL != "wss://example.com/ws" {
		t.Errorf("ServerURL = %q, want %q", cfg.ServerURL, "wss://example.com/ws")
	}
	if cfg.ServerToken != "secret" {
		t.Errorf("ServerToken = %q, want %q", cfg.ServerToken, "secret")
	}
	if cfg.HeartbeatIntervalSecs != 45 {
		t.Errorf("HeartbeatIntervalSecs = %d, want 45", cfg.HeartbeatIntervalSecs)
	}
	if cfg.NodeID == "" {
		t.Error("NodeID should be non-empty (auto-derived from hardware)")
	}
}

// TestLoadNodeConfigDefaults verifies that HeartbeatIntervalSecs defaults to 30 when unset.
func TestLoadNodeConfigDefaults(t *testing.T) {
	t.Setenv("SERVER_URL", "wss://example.com/ws")
	t.Setenv("SERVER_TOKEN", "secret")
	t.Setenv("HEARTBEAT_INTERVAL_SECS", "")

	cfg, err := LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig() returned error: %v", err)
	}

	if cfg.HeartbeatIntervalSecs != 30 {
		t.Errorf("HeartbeatIntervalSecs = %d, want 30 (default)", cfg.HeartbeatIntervalSecs)
	}
}

// TestLoadNodeConfigMissingURL verifies that missing SERVER_URL returns an error.
func TestLoadNodeConfigMissingURL(t *testing.T) {
	t.Setenv("SERVER_URL", "")
	t.Setenv("SERVER_TOKEN", "secret")
	t.Setenv("HEARTBEAT_INTERVAL_SECS", "")

	_, err := LoadNodeConfig()
	if err == nil {
		t.Fatal("LoadNodeConfig() should return error when SERVER_URL is empty")
	}
	if !strings.Contains(err.Error(), "SERVER_URL") {
		t.Errorf("error should mention SERVER_URL, got: %v", err)
	}
}

// TestLoadNodeConfigMissingToken verifies that missing SERVER_TOKEN returns an error.
func TestLoadNodeConfigMissingToken(t *testing.T) {
	t.Setenv("SERVER_URL", "wss://example.com/ws")
	t.Setenv("SERVER_TOKEN", "")
	t.Setenv("HEARTBEAT_INTERVAL_SECS", "")

	_, err := LoadNodeConfig()
	if err == nil {
		t.Fatal("LoadNodeConfig() should return error when SERVER_TOKEN is empty")
	}
	if !strings.Contains(err.Error(), "SERVER_TOKEN") {
		t.Errorf("error should mention SERVER_TOKEN, got: %v", err)
	}
}

// TestLoadNodeConfigInvalidHeartbeat verifies that a non-numeric HEARTBEAT_INTERVAL_SECS returns an error.
func TestLoadNodeConfigInvalidHeartbeat(t *testing.T) {
	t.Setenv("SERVER_URL", "wss://example.com/ws")
	t.Setenv("SERVER_TOKEN", "secret")
	t.Setenv("HEARTBEAT_INTERVAL_SECS", "notanumber")

	_, err := LoadNodeConfig()
	if err == nil {
		t.Fatal("LoadNodeConfig() should return error when HEARTBEAT_INTERVAL_SECS is not a number")
	}
	if !strings.Contains(err.Error(), "HEARTBEAT_INTERVAL_SECS") {
		t.Errorf("error should mention HEARTBEAT_INTERVAL_SECS, got: %v", err)
	}
}

// TestLoadNodeConfigRejectsInsecureWS verifies that LoadNodeConfig rejects ws:// URLs.
func TestLoadNodeConfigRejectsInsecureWS(t *testing.T) {
	t.Setenv("SERVER_URL", "ws://example.com/ws")
	t.Setenv("SERVER_TOKEN", "secret")
	t.Setenv("HEARTBEAT_INTERVAL_SECS", "")

	_, err := LoadNodeConfig()
	if err == nil {
		t.Fatal("LoadNodeConfig() should return error when SERVER_URL uses ws:// scheme")
	}
	if !strings.Contains(err.Error(), "wss://") {
		t.Errorf("error should mention wss://, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ws://") {
		t.Errorf("error should mention the rejected scheme ws://, got: %v", err)
	}
}

// TestLoadNodeConfigProjects verifies that PROJECTS="alpha, beta,gamma" produces []string{"alpha", "beta", "gamma"}.
func TestLoadNodeConfigProjects(t *testing.T) {
	t.Setenv("SERVER_URL", "wss://example.com/ws")
	t.Setenv("SERVER_TOKEN", "secret")
	t.Setenv("PROJECTS", "alpha, beta,gamma")

	cfg, err := LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig() returned error: %v", err)
	}

	if len(cfg.Projects) != 3 {
		t.Fatalf("Projects len = %d, want 3; got %v", len(cfg.Projects), cfg.Projects)
	}
	if cfg.Projects[0] != "alpha" {
		t.Errorf("Projects[0] = %q, want %q", cfg.Projects[0], "alpha")
	}
	if cfg.Projects[1] != "beta" {
		t.Errorf("Projects[1] = %q, want %q", cfg.Projects[1], "beta")
	}
	if cfg.Projects[2] != "gamma" {
		t.Errorf("Projects[2] = %q, want %q", cfg.Projects[2], "gamma")
	}
}

// TestLoadNodeConfigProjectsEmpty verifies that PROJECTS="" produces a non-nil empty slice.
func TestLoadNodeConfigProjectsEmpty(t *testing.T) {
	t.Setenv("SERVER_URL", "wss://example.com/ws")
	t.Setenv("SERVER_TOKEN", "secret")
	t.Setenv("PROJECTS", "")

	cfg, err := LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig() returned error: %v", err)
	}

	if cfg.Projects == nil {
		t.Error("Projects should not be nil when PROJECTS is empty — want []string{}")
	}
	if len(cfg.Projects) != 0 {
		t.Errorf("Projects len = %d, want 0; got %v", len(cfg.Projects), cfg.Projects)
	}
}

// TestLoadNodeConfigProjectsSingleItem verifies that PROJECTS="solo" produces []string{"solo"}.
func TestLoadNodeConfigProjectsSingleItem(t *testing.T) {
	t.Setenv("SERVER_URL", "wss://example.com/ws")
	t.Setenv("SERVER_TOKEN", "secret")
	t.Setenv("PROJECTS", "solo")

	cfg, err := LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig() returned error: %v", err)
	}

	if len(cfg.Projects) != 1 {
		t.Fatalf("Projects len = %d, want 1; got %v", len(cfg.Projects), cfg.Projects)
	}
	if cfg.Projects[0] != "solo" {
		t.Errorf("Projects[0] = %q, want %q", cfg.Projects[0], "solo")
	}
}

// TestLoadNodeConfigNoTelegramRequired verifies that LoadNodeConfig succeeds without Telegram env vars.
func TestLoadNodeConfigNoTelegramRequired(t *testing.T) {
	t.Setenv("SERVER_URL", "wss://example.com/ws")
	t.Setenv("SERVER_TOKEN", "secret")
	t.Setenv("HEARTBEAT_INTERVAL_SECS", "")
	// Explicitly ensure Telegram vars are not set (they should not be required)
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_ALLOWED_USERS", "")

	_, err := LoadNodeConfig()
	if err != nil {
		t.Fatalf("LoadNodeConfig() should not require Telegram vars, but got error: %v", err)
	}
}
