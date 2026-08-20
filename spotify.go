package main

import (
	"os/exec"
	"strings"
)

// spotify.go — "now playing" from the local Spotify session (D-Bus via
// playerctl or dbus-send). Read-only. Best-effort; empty on no player.

type NowPlaying struct {
	Title    string
	Artist   string
	Playing  bool
	Progress string
	Err      string
}

func collectSpotify() NowPlaying {
	var np NowPlaying

	// Prefer playerctl (playerctld may expose spotify).
	out, err := exec.Command("playerctl", "-p", "spotify", "metadata", "--format", "{{ title }}|{{ artist }}|{{ status }}|{{ position }}/{{ mpris:length }}").Output()
	if err != nil {
		np.Err = "playerctl: no spotify player"
		return np
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) >= 3 {
		np.Title = parts[0]
		np.Artist = parts[1]
		np.Playing = parts[2] == "Playing"
		if len(parts) >= 4 {
			np.Progress = parts[3]
		}
	}
	return np
}
