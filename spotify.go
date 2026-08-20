package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// spotify.go — "now playing" from the local Spotify MPRIS session via busctl
// (no playerctl dependency). Read-only. Empty on no player.

type NowPlaying struct {
	Title    string
	Artist   string
	Playing  bool
	Position int64 // microseconds
	Length   int64 // microseconds
	Err      string
}

func collectSpotify() NowPlaying {
	var np NowPlaying

	get := func(prop string) string {
		out, err := exec.Command("busctl", "--user", "get-property",
			"org.mpris.MediaPlayer2.spotify", "/org/mpris/MediaPlayer2",
			"org.mpris.MediaPlayer2.Player", prop).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}

	md := get("Metadata")
	if md == "" {
		np.Err = "no spotify player"
		return np
	}

	// Parse the dict for xesam:title / xesam:artist (busctl prints a verbose dict).
	title := extractMpris(md, "xesam:title")
	artist := extractMpris(md, "xesam:artist")
	np.Title = title
	np.Artist = artist

	// Status: "playing" / "paused".
	status := get("PlaybackStatus")
	np.Playing = strings.Contains(status, "Playing")

	// Position + length (microseconds).
	if pos := get("Position"); pos != "" {
		// busctl returns "t 123456" — take the number.
		f := strings.Fields(pos)
		if len(f) >= 2 {
			np.Position, _ = strconv.ParseInt(f[1], 10, 64)
		}
	}
	// Length comes from the metadata dict (mpris:length).
	np.Length = extractMprisInt(md, "mpris:length")

	return np
}

// extractMpris pulls a string value for a key from a busctl metadata dict dump.
// Format: `"key" s "value"` or `"key" as 1 "value"`. After the key, the
// structure is: closing quote, type sigil, then "value". The value is the
// third quote-delimited token — scan past the first two quotes.
func extractMpris(dict, key string) string {
	idx := strings.Index(dict, key)
	if idx < 0 {
		return ""
	}
	rest := dict[idx+len(key):]
	// Skip the key's closing quote + any type sigil: advance past 2 quotes.
	quotes := 0
	for i := 0; i < len(rest); i++ {
		if rest[i] == '"' {
			quotes++
			if quotes == 2 {
				rest = rest[i+1:]
				if end := strings.Index(rest, `"`); end >= 0 {
					return rest[:end]
				}
				return ""
			}
		}
	}
	return ""
}

// extractMprisInt pulls a numeric value for a key (mpris:length is "x 123").
func extractMprisInt(dict, key string) int64 {
	idx := strings.Index(dict, key)
	if idx < 0 {
		return 0
	}
	rest := dict[idx+len(key):]
	// Find the first number after the key.
	for i := 0; i < len(rest); i++ {
		if rest[i] >= '0' && rest[i] <= '9' {
			j := i
			for j < len(rest) && (rest[j] >= '0' && rest[j] <= '9') {
				j++
			}
			v, _ := strconv.ParseInt(rest[i:j], 10, 64)
			return v
		}
	}
	return 0
}
