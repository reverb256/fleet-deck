package main

import (
	"strings"
)

// ai.go — read-only AI inference activity across the cluster: running model
// servers (llama-swap, vllm, llamafile) and any local inference processes.
// Queries HTTP endpoints where present; never starts or stops anything.

type ModelServer struct {
	Host  string
	Name  string // llama-swap / vllm / llamafile
	Model string // current loaded model
	State string // running / idle / error
}

type AIState struct {
	Servers []ModelServer
	Procs   []string // inference processes (pid, cmd tail)
	Err     string
}

// Endpoints we know about (llama-swap proxies per host):
// nexus listen=21759 / forge listen=21763 / sentry listen=21764.
var aiEndpoints = []struct{ host, port string }{
	{"nexus", "21759"},
	{"sentry", "21764"},
	{"forge", "21763"},
}

func collectAI() AIState {
	var s AIState

	// 1. Probe known model-server HTTP endpoints (read-only GET /v1/models).
	for _, ep := range aiEndpoints {
		probe := `curl -s -m 2 http://127.0.0.1:` + ep.port + `/v1/models 2>/dev/null | head -c 2000`
		out, err := runSSH(Host{Name: ep.host, Address: ep.host}, probe)
		if err != nil {
			continue
		}
		ms := ModelServer{Host: ep.host, State: "idle"}
		if strings.Contains(out, "llama-swap") {
			ms.Name = "llama-swap"
		} else if strings.Contains(out, "vllm") {
			ms.Name = "vllm"
		} else {
			ms.Name = "openai-compat"
		}
		// Extract every model id from the JSON array (best-effort).
		ms.Model = extractModelIDs(out)
		s.Servers = append(s.Servers, ms)
	}

	// 2. Scan for inference processes (llama.cpp / llamafile / python vllm).
	procCmd := `bash --norc --noprofile -c 'ps -eo pid,etime,cmd | grep -iE "llama|vllm|llamafile|text-generation|tgi|ollama" | grep -v grep | head -15'`
	for _, host := range []string{"sentry", "nexus", "zephyr"} {
		out, err := runSSH(Host{Name: host, Address: host}, procCmd)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if line != "" {
				s.Procs = append(s.Procs, host+": "+line)
			}
		}
	}

	return s
}

// extractModelIDs pulls every "id" value from an OpenAI-style /v1/models
// response ({"data":[{"id":...},...]}). Best-effort, comma-joined.
func extractModelIDs(json string) string {
	var ids []string
	for {
		idx := strings.Index(json, `"id"`)
		if idx < 0 {
			break
		}
		rest := json[idx:]
		colon := strings.Index(rest, ":")
		if colon < 0 {
			break
		}
		val := strings.TrimSpace(rest[colon+1:])
		val = strings.TrimPrefix(val, `"`)
		if end := strings.Index(val, `"`); end >= 0 {
			ids = append(ids, val[:end])
			json = rest[colon+1:]
		} else {
			break
		}
	}
	return strings.Join(ids, ", ")
}
