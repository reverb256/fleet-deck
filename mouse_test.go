package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMouseClickSwitchesTab(t *testing.T) {
	m := initialModel()
	m.width = 200
	m.height = 50

	// Click on the "mining" tab (46..55 per debug output)
	click := tea.MouseMsg{Type: tea.MouseLeft, X: 50, Y: 0}
	updated, _ := m.Update(click)
	nm := updated.(model)
	if nm.activeTab != 4 {
		t.Fatalf("expected activeTab=4 (mining), got %d", nm.activeTab)
	}

	// Click on hosts tab (x=16)
	click2 := tea.MouseMsg{Type: tea.MouseLeft, X: 16, Y: 0}
	updated, _ = nm.Update(click2)
	nm = updated.(model)
	if nm.activeTab != 1 {
		t.Fatalf("expected activeTab=1 (hosts), got %d", nm.activeTab)
	}
}
