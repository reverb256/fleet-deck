package main

import (
	"strconv"
	"strings"
)

// mining.go — read-only mining fleet status: per-GPU temp, power, util, and
// hashrate where available. Polls nvidia-smi on mining hosts over ssh.

type MiningHost struct {
	Name  string
	GPUs  []GPUInfo
	TotalHash float64 // MH/s when detectable
	HashUnit string
	Err   string
}

type MiningState struct {
	Hosts []MiningHost
}

// miningHosts: forge (2x 4060) + zephyr (3090) + sentry mine.
var miningHostNames = []string{"forge", "zephyr", "sentry"}

func collectMining() MiningState {
	var s MiningState
	for _, name := range miningHostNames {
		h := MiningHost{Name: name}
		gpus, err := collectGPU(Host{Name: name, Address: name})
		if err != nil {
			h.Err = err.Error()
			s.Hosts = append(s.Hosts, h)
			continue
		}
		h.GPUs = gpus

		// Hashrate: try miner CLI endpoints (peakminer/trex). Best-effort.
		h.TotalHash, h.HashUnit = collectHashrate(name)
		s.Hosts = append(s.Hosts, h)
	}
	return s
}

func collectHashrate(host string) (float64, string) {
	// Peakminer CLI / API: try common binaries; read-only, 2s timeout.
	cmds := []string{
		`bash --norc --noprofile -c 'peakminer-cli stats 2>/dev/null | head -20'`,
		`bash --norc --noprofile -c 'miner hash-rate 2>/dev/null | head -20'`,
		`bash --norc --noprofile -c 'nvidia-smi --query-gpu=clocks.sm --format=csv,noheader,nounits 2>/dev/null'`,
	}
	for _, c := range cmds {
		out, err := runSSH(Host{Name: host, Address: host}, c)
		if err != nil || strings.TrimSpace(out) == "" {
			continue
		}
		// crude parse: look for a number followed by MH/s / KH/s / GH/s
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "MH/s") || strings.Contains(line, "KH/s") || strings.Contains(line, "GH/s") {
				f := strings.Fields(line)
				for _, tok := range f {
					if v, err := strconv.ParseFloat(tok, 64); err == nil {
						unit := "MH/s"
						if strings.Contains(line, "KH/s") {
							unit = "KH/s"
						} else if strings.Contains(line, "GH/s") {
							unit = "GH/s"
						}
						return v, unit
					}
				}
			}
		}
	}
	return 0, ""
}
