package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// NodeConfig holds configuration for the node's WebSocket connection to the server.
// Separate from Config to avoid requiring Telegram env vars.
type NodeConfig struct {
	// ServerURL is the WebSocket server URL (wss://...) the node connects to. Required.
	ServerURL string

	// ServerToken is the bearer token for server authentication. Required.
	ServerToken string

	// HeartbeatIntervalSecs is the WebSocket ping interval in seconds. Default: 30.
	HeartbeatIntervalSecs int

	// NodeID is the hardware-derived node identifier (auto-populated, not from env).
	NodeID string
}

// LoadNodeConfig reads node configuration from environment variables (and .env if present).
// Required: SERVER_URL, SERVER_TOKEN.
// Optional: HEARTBEAT_INTERVAL_SECS (default: 30).
// NodeID is auto-derived from hardware identifiers.
func LoadNodeConfig() (*NodeConfig, error) {
	_ = godotenv.Load()

	cfg := &NodeConfig{}

	// --- Required: SERVER_URL ---
	cfg.ServerURL = os.Getenv("SERVER_URL")
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("SERVER_URL environment variable is required")
	}

	// --- Required: SERVER_TOKEN ---
	cfg.ServerToken = os.Getenv("SERVER_TOKEN")
	if cfg.ServerToken == "" {
		return nil, fmt.Errorf("SERVER_TOKEN environment variable is required")
	}

	// --- HEARTBEAT_INTERVAL_SECS (default: 30) ---
	cfg.HeartbeatIntervalSecs = 30
	if v := os.Getenv("HEARTBEAT_INTERVAL_SECS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid HEARTBEAT_INTERVAL_SECS: %q", v)
		}
		cfg.HeartbeatIntervalSecs = n
	}

	// --- NodeID (auto-derived, not from env) ---
	cfg.NodeID = DeriveNodeID()

	return cfg, nil
}
