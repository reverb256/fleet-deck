package main

import (
	"strings"
)

// k3s.go — read-only k3s cluster state (nodes, pods) collected over ssh to
// the control-plane host (nexus). Never mutates the cluster.

type K3sState struct {
	Nodes []string // "NAME STATUS ROLES AGE VERSION"
	Pods  []string // "NAMESPACE NAME READY STATUS RESTARTS AGE"
	Err   string
}

// collectK3s runs kubectl on the control plane (nexus) and parses nodes+pods.
func collectK3s() K3sState {
	var s K3sState

	out, err := runSSH(Host{Name: "nexus", Address: "nexus"},
		`bash --norc --noprofile -c 'kubectl --kubeconfig /etc/rancher/k3s/k3s.yaml get nodes 2>&1 | tail -n +2; echo "---PODS---"; kubectl --kubeconfig /etc/rancher/k3s/k3s.yaml get pods -A 2>&1 | tail -n +2'`)
	if err != nil || len(nonEmpty(strings.Split(out, "\n"))) == 0 {
		// Fallback: try the user default config.
		out, err = runSSH(Host{Name: "nexus", Address: "nexus"},
			`bash --norc --noprofile -c 'kubectl get nodes 2>&1 | tail -n +2; echo "---PODS---"; kubectl get pods -A 2>&1 | tail -n +2'`)
		if err != nil {
			s.Err = "kubectl: " + err.Error()
			return s
		}
	}

	// Split the two sections.
	podPart := ""
	if i := strings.Index(out, "---PODS---"); i >= 0 {
		s.Nodes = nonEmpty(strings.Split(out[:i], "\n"))
		podPart = out[i+len("---PODS---"):]
	} else {
		s.Nodes = nonEmpty(strings.Split(out, "\n"))
	}
	s.Pods = nonEmpty(strings.Split(podPart, "\n"))

	// If kubectl emitted only error lines (API down), surface a clean message.
	cleanNodes := filterKubectlErrors(s.Nodes)
	s.Nodes = cleanNodes

	return s
}

// filterKubectlErrors drops the noisy memcache.go stack lines and keeps a
// single meaningful error if the API is unreachable.
func filterKubectlErrors(lines []string) []string {
	var kept []string
	sawError := false
	for _, l := range lines {
		if strings.Contains(l, "memcache.go") || strings.HasPrefix(l, "E0820") {
			if !sawError && strings.Contains(l, "Unable to connect") {
				kept = append(kept, l)
				sawError = true
			}
			continue
		}
		kept = append(kept, l)
	}
	return kept
}

func nonEmpty(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}
