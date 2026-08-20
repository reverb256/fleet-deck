package main

import (
	"os"
	"testing"
)

// skipIfNotLive skips live-cluster tests unless FLEET_DECK_LIVE=1.
// These hit real hosts over ssh and must NOT run inside the Nix build
// sandbox (no ssh binary, no network).
func skipIfNotLive(t *testing.T) {
	t.Helper()
	if os.Getenv("FLEET_DECK_LIVE") != "1" {
		t.Skip("live cluster test — set FLEET_DECK_LIVE=1 to run")
	}
}
