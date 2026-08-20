package main

import (
	"testing"
	"time"
)

// Verify full collection on all hosts (with GPU on forge/zephyr).
func TestFullCollection(t *testing.T) {
	for _, name := range []string{"zephyr", "nexus", "sentry", "forge"} {
		_, _ = poolFor(name).run("echo warm", 3*time.Second)
		start := time.Now()
		h := collectHost(Host{Name: name, Address: name, HasGPU: name != "nexus"})
		elapsed := time.Since(start)
		t.Logf("%-7s cpu=%.1f mem=%.1f/%.1f disk=%.1f/%.1f netrx=%.0f temp=%.0f gpu=%d builds=%d err=%q (%v)",
			name, h.CPU, h.MemUsed, h.MemTotal, h.DiskUsed, h.DiskTotal, h.NetRX, h.Temp, len(h.GPU), len(h.Builds), h.Err, elapsed)
		if elapsed > 2*time.Second {
			t.Errorf("%s too slow: %v", name, elapsed)
		}
	}
}
