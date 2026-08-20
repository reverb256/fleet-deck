package main

import "testing"

func TestCollectAI(t *testing.T) {
	s := collectAI()
	t.Logf("servers=%d procs=%d", len(s.Servers), len(s.Procs))
	for _, sv := range s.Servers {
		t.Logf("  %s %s %s %s", sv.Host, sv.Name, sv.Model, sv.State)
	}
	for _, p := range s.Procs {
		t.Logf("  proc: %s", p)
	}
}

func TestCollectK3s(t *testing.T) {
	k := collectK3s()
	t.Logf("nodes=%d pods=%d err=%q", len(k.Nodes), len(k.Pods), k.Err)
	for _, n := range k.Nodes {
		t.Logf("  node: %s", n)
	}
}
