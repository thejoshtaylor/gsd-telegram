package config

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/denisbrodbeck/machineid"
)

// DeriveNodeID returns a stable, transmittable node identifier.
// Primary: machineid.ProtectedID("gsd-node") — HMAC-SHA256 of OS machine UUID.
// Fallback: sha256(hostname)[:16 hex chars] when machineid fails (containers, CI).
func DeriveNodeID() string {
	id, err := machineid.ProtectedID("gsd-node")
	if err == nil && id != "" {
		return id
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown"
	}
	sum := sha256.Sum256([]byte(host))
	return fmt.Sprintf("%x", sum[:8])
}
