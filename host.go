package main

import (
	"strconv"
	"strings"
	"time"
)

// host.go — single-round-trip metric collection.
//
// One ssh call per host returns ALL metrics as a key=value blob. No more
// 4+ ssh round-trips per host per tick. CPU comes from /proc/stat deltas
// (two reads 500ms apart — a tiny file read, not a full top process scan).
//
// Remote protocol (the entire blob is emitted by ONE bash invocation):
//
//	cpu=N                # percent, computed from /proc/stat delta
//	mem=used/total       # MB
//	disk=used/total      # MB
//	netrx=N nett tx=N    # cumulative bytes from sysfs
//	temp=N               # CPU temp if available
//	gpu=<nvidia-smi csv> # optional, one line per GPU
//	builds=<ps lines>    # optional
//
// Everything is read-only. Never starts builds or mutates the host.

const remoteScript = `
export PATH=/run/current-system/sw/bin:/usr/bin:/bin
# CPU% from /proc/stat delta (two samples 500ms apart — cheap, no top).
c1=$(awk '/^cpu /{print $2+$3+$4+$5+$6+$7+$8; print $5+$6+$7+$8}' /proc/stat)
sleep 0.5
c2=$(awk '/^cpu /{print $2+$3+$4+$5+$6+$7+$8; print $5+$6+$7+$8}' /proc/stat)
set -- $c1; t1=$1; i1=$2
set -- $c2; t2=$1; i2=$2
dt=$((t2-t1)); di=$((i2-i1))
if [ "$dt" -gt 0 ]; then cpu=$((100 - di*100/dt)); else cpu=0; fi
[ "$cpu" -lt 0 ] && cpu=0; [ "$cpu" -gt 100 ] && cpu=100
echo "cpu=$cpu"

# Mem (MB): MemTotal and MemAvailable from /proc/meminfo.
mt=$(awk '/^MemTotal:/ {print $2}' /proc/meminfo)
ma=$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo)
[ -z "$mt" ] && mt=0; [ -z "$ma" ] && ma=0
mu=$(( (mt - ma) / 1024 ))
echo "mem=$mu/$((mt/1024))"

# Disk (MB) on root.
read -r du dt2 < <(df -m / | awk 'NR==2 {print $3, $2}')
echo "disk=$du/$dt2"

# Net: first physical (non-virtual) iface via sysfs.
for i in /sys/class/net/*; do
  n=${i##*/}
  case "$n" in lo|veth*|cali*|tailscale*|podman*|vxlan*|br-*|docker*|flannel*) ;; *)
    rx=$(cat "$i/statistics/rx_bytes" 2>/dev/null || echo 0)
    tx=$(cat "$i/statistics/tx_bytes" 2>/dev/null || echo 0)
    echo "netrx=$rx"
    echo "nett=$tx"
    break
    ;;
  esac
done

# CPU temp — any thermal zone, first that reports a sane value.
t=0
for z in /sys/class/thermal/thermal_zone*/temp; do
  [ -r "$z" ] || continue
  tv=$(cat "$z" 2>/dev/null || echo 0)
  if [ "$tv" -gt 1000 ] 2>/dev/null; then t=$((tv/1000)); break; fi
done
echo "temp=$t"

# GPU (nvidia-smi, one line per GPU) — only if nvidia-smi exists.
if command -v nvidia-smi >/dev/null 2>&1; then
  nvidia-smi --query-gpu=index,name,utilization.gpu,temperature.gpu,power.draw,memory.used --format=csv,noheader,nounits 2>/dev/null | while IFS=, read -r gi gn gu gt gp gm; do
    echo "gpu=$gi|$gn|$gu|$gt|$gp|$gm"
  done
fi

# Builds (nix/colmena processes).
ps -eo pid,etime,cmd 2>/dev/null | grep -iE 'nix build|nixos-rebuild|colmena|__build-remote|nix-store.*realise' | grep -v grep | head -20 | while IFS= read -r bl; do
  echo "build=$bl"
done
`

// buildRemoteCmd wraps remoteScript in an explicit bash invocation (host
// default shell may be fish). Single quotes in the script are escaped for
// the bash -c string.
func buildRemoteCmd() string {
	return "bash --norc --noprofile -c '" + strings.ReplaceAll(remoteScript, "'", `'"'"'`) + "'"
}

// collectHost gathers live metrics for a host in ONE ssh round-trip.
func collectHost(h Host) Host {
	// Track the collection duration for the latency footer.
	start := time.Now()

	out, err := poolFor(h.Address).run(buildRemoteCmd(), 4*time.Second)
	if err != nil {
		h.Err = err.Error()
		return h
	}

	vals := map[string][]string{}
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, "="); i > 0 {
			k := line[:i]
			vals[k] = append(vals[k], line[i+1:])
		}
	}

	// CPU
	if v := first(vals, "cpu"); v != "" {
		h.CPU, _ = strconv.ParseFloat(v, 64)
	}
	// Mem (MB → GB)
	if v := first(vals, "mem"); v != "" {
		parts := strings.SplitN(v, "/", 2)
		if len(parts) == 2 {
			u, _ := strconv.ParseFloat(parts[0], 64)
			t, _ := strconv.ParseFloat(parts[1], 64)
			h.MemUsed = u / 1024
			h.MemTotal = t / 1024
		}
	}
	// Disk (MB → GB)
	if v := first(vals, "disk"); v != "" {
		parts := strings.SplitN(v, "/", 2)
		if len(parts) == 2 {
			u, _ := strconv.ParseFloat(parts[0], 64)
			t, _ := strconv.ParseFloat(parts[1], 64)
			h.DiskUsed = u / 1024
			h.DiskTotal = t / 1024
		}
	}
	// Net (cumulative bytes from sysfs)
	if v := first(vals, "netrx"); v != "" {
		h.NetRX, _ = strconv.ParseFloat(v, 64)
	}
	if v := first(vals, "nett"); v != "" {
		h.NetTX, _ = strconv.ParseFloat(v, 64)
	}
	// Temp
	if v := first(vals, "temp"); v != "" {
		h.Temp, _ = strconv.ParseFloat(v, 64)
	}
	// GPU
	for _, v := range vals["gpu"] {
		f := strings.Split(v, "|")
		if len(f) < 6 {
			continue
		}
		idx, _ := strconv.Atoi(f[0])
		util, _ := strconv.ParseFloat(f[2], 64)
		temp, _ := strconv.ParseFloat(f[3], 64)
		power, _ := strconv.ParseFloat(f[4], 64)
		mem, _ := strconv.ParseFloat(f[5], 64)
		h.GPU = append(h.GPU, GPUInfo{
			Index: idx,
			Name:  f[1],
			Util:  util,
			Temp:  temp,
			Power: power,
			Mem:   mem,
		})
	}
	// Builds
	for _, v := range vals["build"] {
		f := strings.Fields(v)
		if len(f) < 3 {
			continue
		}
		tail := strings.Join(f[2:], " ")
		if len(tail) > 55 {
			tail = tail[:55] + "…"
		}
		h.Builds = append(h.Builds, f[0]+" "+f[1]+" "+tail)
	}

	h.latencyMs = time.Since(start).Milliseconds()
	return h
}

func first(m map[string][]string, k string) string {
	if v := m[k]; len(v) > 0 {
		return v[0]
	}
	return ""
}
