package main

import (
	"os"
	"path/filepath"
	"strings"
)

// scratch.go — the AI-thoughts scratchboard: a persistent append-only note
// buffer stored at ~/.fleet-deck/thoughts.md. Editable in the TUI.

const scratchDir = ".fleet-deck"

func scratchPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/fleet-deck-thoughts.md"
	}
	return filepath.Join(home, scratchDir, "thoughts.md")
}

func loadScratch() []string {
	data, err := os.ReadFile(scratchPath())
	if err != nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func appendScratch(line string) error {
	f, err := os.OpenFile(scratchPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}
