package config

import "testing"

// TestDeriveNodeID verifies that DeriveNodeID returns a non-empty string of adequate length.
func TestDeriveNodeID(t *testing.T) {
	id := DeriveNodeID()
	if id == "" {
		t.Fatal("DeriveNodeID() returned empty string")
	}
	if len(id) < 16 {
		t.Errorf("DeriveNodeID() = %q (len %d), want len >= 16", id, len(id))
	}
}

// TestDeriveNodeIDStable verifies that DeriveNodeID returns the same value on repeated calls.
func TestDeriveNodeIDStable(t *testing.T) {
	id1 := DeriveNodeID()
	id2 := DeriveNodeID()
	if id1 != id2 {
		t.Errorf("DeriveNodeID() not stable: %q != %q", id1, id2)
	}
}
