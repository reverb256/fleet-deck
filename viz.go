package main

import (
	"strings"
)

// viz.go — character-resolution data viz (tui-design §5).
// Sparklines (▁▂▃▅▇█), gauges ([████░░] 67%), shade blocks.

// sparkChars maps a 0..1 value to an 8-height sparkline char.
var sparkChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// sparkline renders a series of 0..100 values as an 8-height sparkline.
func sparkline(vals []float64, width int) string {
	if len(vals) == 0 || width <= 0 {
		return ""
	}
	// Downsample/expand to exactly `width` cells.
	out := make([]string, 0, width)
	n := len(vals)
	for i := 0; i < width; i++ {
		// Map output cell i to input index j (interpolate).
		j := i * (n - 1) / maxInt(1, width-1)
		v := vals[j]
		idx := int(v / 100.0 * 7.99)
		if idx < 0 {
			idx = 0
		}
		if idx > 7 {
			idx = 7
		}
		out = append(out, sparkChars[idx])
	}
	return strings.Join(out, "")
}

// gauge renders [██████░░░░] with percentage.
// Colors: green→yellow→red by value (tui-design progress bar rule).
func gauge(pct float64, width int) string {
	if width <= 0 {
		width = 10
	}
	filled := int(pct / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	// Color by threshold.
	style := styleOK
	if pct >= 75 {
		style = styleErr
	} else if pct >= 50 {
		style = styleWarn
	}
	return style.Render("[" + bar + "]")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// brailleGraph renders a series of 0..100 values as a 2-row braille line
// (btop-style). Each braille cell encodes 2 vertical dots per column.
var braillePatterns = []string{
	" ", "⢀", "⢠", "⢰", "⢸", // top-dot column (down to up)
	"⡀", "⣀", "⣠", "⣰", "⣸", "⡄", "⣄", "⣤", "⣴", "⣼",
	"⡆", "⣆", "⣦", "⣶", "⣾", "⡇", "⣇", "⣧", "⣷", "⣿",
}

// brailleLine renders vals (0..100) as a single braille row of `width` cells.
// Each cell: bottom dot for low value, top dot for high value (like btop).
func brailleLine(vals []float64, width int) string {
	if len(vals) == 0 || width <= 0 {
		return strings.Repeat(" ", width)
	}
	n := len(vals)
	out := make([]string, 0, width)
	for i := 0; i < width; i++ {
		// Map output cell to input index; sample at i/(width-1) through series.
		j := i * (n - 1) / maxInt(1, width-1)
		v := vals[j]
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		// Map 0..100 → braille char (bottom→top).
		idx := int(v / 100.0 * 11.99)
		if idx >= len(braillePatterns) {
			idx = len(braillePatterns) - 1
		}
		out = append(out, braillePatterns[idx])
	}
	return strings.Join(out, "")
}

// btopBar renders a compact colored bar like btop's CPU bar: [██████░░░░]
// with a threshold color. Width 10 default.
func btopBar(pct float64) string {
	return gauge(pct, 10)
}
