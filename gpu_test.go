package main

import (
	"testing"
)

func TestGPUValuesParse(t *testing.T) {
	skipIfNotLive(t)
	h := collectHost(Host{Name: "zephyr", Address: "zephyr", HasGPU: true})
	if len(h.GPU) == 0 {
		t.Fatal("expected GPUs on zephyr")
	}
	for _, g := range h.GPU {
		t.Logf("gpu%d %s util=%.1f temp=%.1f pow=%.1f mem=%.1f", g.Index, g.Name, g.Util, g.Temp, g.Power, g.Mem)
		if g.Util == 0 && g.Temp == 0 {
			t.Errorf("gpu%d values not parsed: util=%.1f temp=%.1f", g.Index, g.Util, g.Temp)
		}
	}
}
