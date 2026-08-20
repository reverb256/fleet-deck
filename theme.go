package main

import (
	"github.com/charmbracelet/lipgloss"
)

// theme.go — Stylix "Osaka Jade" base16 palette mapped to semantic slots.
// Per tui-design: never reference hex in widget code — always semantic slots.
// Colors extracted from the live alacritty config (~/.config/alacritty/alacritty.toml).

// Semantic slots (tui-design §4):
//   fg.default / fg.muted / fg.emphasis / bg.base / bg.surface / bg.selection
//   accent.primary / accent.secondary / status.{error,warning,success,info}

var (
	// bg base / surface / selection
	bgBase     = lipgloss.Color("#111c18")
	bgSurface  = lipgloss.Color("#23372b")
	bgSel      = lipgloss.Color("#3a4f43")
	// fg
	fgDefault  = lipgloss.Color("#c1c497")
	fgMuted    = lipgloss.Color("#3a4f43")
	fgEmphasis = lipgloss.Color("#f6f5dd")
	// accents
	accentPrimary   = lipgloss.Color("#2dd5b7") // cyan
	accentSecondary = lipgloss.Color("#d2689c") // magenta
	// status
	statusError   = lipgloss.Color("#ff5345")
	statusWarning = lipgloss.Color("#e5c736")
	statusSuccess = lipgloss.Color("#549e6a")
	statusInfo    = lipgloss.Color("#2dd5b7")
)

// Semantic styles used across widgets.
var (
	styleDefault = lipgloss.NewStyle().Foreground(fgDefault)
	styleMuted   = lipgloss.NewStyle().Foreground(fgMuted)
	styleEmph    = lipgloss.NewStyle().Foreground(fgEmphasis).Bold(true)
	styleErr     = lipgloss.NewStyle().Foreground(statusError)
	styleWarn    = lipgloss.NewStyle().Foreground(statusWarning)
	styleOK      = lipgloss.NewStyle().Foreground(statusSuccess)
	styleInfo    = lipgloss.NewStyle().Foreground(statusInfo)
	styleAccent  = lipgloss.NewStyle().Foreground(accentPrimary).Bold(true)

	// Panel chrome — minimal, serves content (anti-pattern #10).
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(bgSel).
			Padding(0, 1)
	panelTitleStyle = func(s string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(accentPrimary).Bold(true)
	}
	// Alias kept for render code.
	panelTitle = panelTitleStyle
	statusStyle = lipgloss.NewStyle().Foreground(fgMuted)
	tabActive   = lipgloss.NewStyle().Foreground(bgBase).Background(accentPrimary).Bold(true).Padding(0, 1)
	tabIdle     = lipgloss.NewStyle().Foreground(fgMuted).Padding(0, 1)
)

// Aliases so existing render code keeps working with clearer names.
var (
	themeBg     = bgBase
	themeFg     = fgDefault
	themeAccent = accentPrimary
	themePink   = accentSecondary
	themeGreen  = statusSuccess
	themeRed    = statusError
	themeYellow = statusWarning
	themeCyan   = statusInfo
	themeDim    = bgSel
	themeBright = fgEmphasis
	hostListStyle = lipgloss.NewStyle().
			Width(30).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(bgSel).
			Padding(0, 1)
	overviewStyle = lipgloss.NewStyle().
			Width(110).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(bgSel).
			Padding(0, 1)
	buildsStyle = lipgloss.NewStyle().
			Width(60).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(bgSel).
			Padding(0, 1)
)
