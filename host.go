package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// collectHost gathers live metrics for a host over ssh. Read-only — never
// starts builds or writes to the host. Returns a Host populated with whatever
// succeeded; Err is set if the ssh itself failed.

func collectHost(h Host) Host {
	// 1. CPU + mem + disk + net in one pass (best-effort single ssh).

	// Simpler, robust: run a small bash that echoes key lines, parse locally.
	remote := `bash --norc --noprofile -c 'echo "cpu=$(top -bn1 | awk '"'"'/Cpu/ {gsub(/%/,""); print 100-$8}'"'"')"; echo "mem=$(free -m | awk '"'"'/^Mem/ {print $3}'"'"')/$(free -m | awk '"'"'/^Mem/ {print $2}'"'"')"; echo "disk=$(df -m / | awk '"'"'NR==2 {print $3}'"'"')/$(df -m / | awk '"'"'NR==2 {print $2}'"'"')"'`

	out, err := runSSH(h, remote)
	if err != nil {
		h.Err = fmt.Sprintf("ssh: %v", err)
		return h
	}

	vals := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, "="); i > 0 {
			vals[line[:i]] = line[i+1:]
		}
	}
	if v, ok := vals["cpu"]; ok {
		h.CPU, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)
	}
	if v, ok := vals["mem"]; ok {
		parts := strings.SplitN(v, "/", 2)
		if len(parts) == 2 {
			u, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			t, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			h.MemUsed = u / 1024
			h.MemTotal = t / 1024
		}
	}
	if v, ok := vals["disk"]; ok {
		parts := strings.SplitN(v, "/", 2)
		if len(parts) == 2 {
			u, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			t, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			h.DiskUsed = u / 1024
			h.DiskTotal = t / 1024
		}
	}

	// Net: read sysfs counters directly (no awk/quoting traps). Find the
	// first non-loopback physical interface and read rx/tx bytes.
	net := collectNetBytes(h)
	h.NetRX, h.NetTX = net.rx, net.tx

	// 2. GPU (nvidia-smi) for mining hosts.
	if h.HasGPU {
		gpus, err := collectGPU(h)
		if err == nil {
			h.GPU = gpus
		}
	}

	// 3. Builds (nix/colmena/remote workers).
	builds, err := collectBuilds(h)
	if err == nil {
		h.Builds = builds
	}

	return h
}

func runSSH(h Host, remote string) (string, error) {
	cmd := exec.Command("ssh", "-o", "ConnectTimeout=3", "-o", "BatchMode=yes", h.Address, remote)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type netCounters struct{ rx, tx float64 }

// collectNetBytes reads rx/tx byte counters from sysfs for the first
// non-loopback, non-virtual physical interface. No awk — plain cat of two
// files, immune to the quoting traps that bit us before.
func collectNetBytes(h Host) netCounters {
	// List real interfaces: exclude lo, veth*, cali*, tailscale*, podman*.
	listCmd := `bash --norc --noprofile -c 'for i in /sys/class/net/*; do n=${i##*/}; case "$n" in lo|veth*|cali*|tailscale*|podman*|vxlan*|br-*|docker*) ;; *) echo "$n";; esac; done | head -1'`
	out, err := runSSH(h, listCmd)
	if err != nil {
		return netCounters{}
	}
	iface := strings.TrimSpace(out)
	if iface == "" {
		return netCounters{}
	}

	rx, err1 := runSSH(h, "cat /sys/class/net/"+iface+"/statistics/rx_bytes")
	tx, err2 := runSSH(h, "cat /sys/class/net/"+iface+"/statistics/tx_bytes")
	var c netCounters
	if err1 == nil {
		c.rx, _ = strconv.ParseFloat(strings.TrimSpace(rx), 64)
	}
	if err2 == nil {
		c.tx, _ = strconv.ParseFloat(strings.TrimSpace(tx), 64)
	}
	return c
}

func collectGPU(h Host) ([]GPUInfo, error) {
	out, err := runSSH(h, `nvidia-smi --query-gpu=index,name,utilization.gpu,temperature.gpu,power.draw,memory.used --format=csv,noheader,nounits 2>/dev/null`)
	if err != nil {
		return nil, err
	}
	var gpus []GPUInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, ",")
		if len(f) < 6 {
			continue
		}
		idx, _ := strconv.Atoi(strings.TrimSpace(f[0]))
		util, _ := strconv.ParseFloat(strings.TrimSpace(f[2]), 64)
		temp, _ := strconv.ParseFloat(strings.TrimSpace(f[3]), 64)
		power, _ := strconv.ParseFloat(strings.TrimSpace(f[4]), 64)
		mem, _ := strconv.ParseFloat(strings.TrimSpace(f[5]), 64)
		gpus = append(gpus, GPUInfo{
			Index: idx,
			Name:  strings.TrimSpace(f[1]),
			Util:  util,
			Temp:  temp,
			Power: power,
			Mem:   mem,
		})
	}
	return gpus, nil
}

func collectBuilds(h Host) ([]string, error) {
	out, err := runSSH(h, `bash --norc --noprofile -c 'ps -eo pid,etime,cmd | grep -iE "nix build|nixos-rebuild|colmena|__build-remote|nix-store.*realise" | grep -v grep'`)
	if err != nil {
		return nil, err
	}
	var builds []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		// Compress: keep pid + etime + a short tail of the command.
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		cmdTail := strings.Join(f[2:], " ")
		if len(cmdTail) > 60 {
			cmdTail = cmdTail[:60] + "…"
		}
		builds = append(builds, fmt.Sprintf("%s %s %s", f[0], f[1], cmdTail))
	}
	return builds, nil
}
