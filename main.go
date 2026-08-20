package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Host model ──────────────────────────────────────────────────────────────
// A cluster host with its metrics, polled over ssh.

type Host struct {
	Name     string
	Address  string
	HasGPU   bool
	CPU      float64
	MemUsed  float64
	MemTotal float64
	NetRX    float64 // bytes since boot (cumulative)
	NetTX    float64
	NetRXRate float64 // Mbps (delta between polls)
	NetTXRate float64
	DiskUsed float64
	DiskTotal float64
	Temp     float64
	GPU      []GPUInfo
	Builds   []string
	latencyMs int64 // collection duration, for the latency footer
	Err      string
}

type GPUInfo struct {
	Index int
	Name  string
	Util  float64
	Mem   float64
	Temp  float64
	Power float64
}

// ── Messages ────────────────────────────────────────────────────────────────

type pollMsg struct {
	host string
	data Host
}

type tickMsg struct{}

// ── Model ───────────────────────────────────────────────────────────────────

type model struct {
	hosts      []Host
	prevHosts  map[string]Host     // for net-rate deltas
	hist       map[string][]float64 // recent CPU% samples per host (braille graphs)
	k3s        K3sState
	mining     MiningState
	nowPlaying NowPlaying
	ai         AIState
	scratch    []string
	scratchBuf string
	showHelp   bool
	polling    bool // single-flight guard: a poll cycle is in flight
	activeTab  int
	width      int
	height     int
	err        string
}

func initialModel() model {
	return model{
		hosts: []Host{
			{Name: "zephyr", Address: "zephyr", HasGPU: true},
			{Name: "nexus", Address: "nexus", HasGPU: false},
			{Name: "sentry", Address: "sentry", HasGPU: true},
			{Name: "forge", Address: "forge", HasGPU: true},
		},
		scratch: loadScratch(),
		hist:    map[string][]float64{},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickEvery(),
		pollAll(m.hosts),
	)
}

func tickEvery() tea.Cmd {
	return tea.Tick(2000, func(t time.Time) tea.Msg { return tickMsg{} })
}

// pollAll returns a batch of polling commands, one per host.
func pollAll(hosts []Host) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(hosts))
	for _, h := range hosts {
		cmds = append(cmds, pollHost(h))
	}
	return tea.Batch(cmds...)
}

func pollHost(h Host) tea.Cmd {
	return func() tea.Msg {
		data := collectHost(h)
		return pollMsg{host: h.Name, data: data}
	}
}

func (m model) cycleDone() bool {
	for _, h := range m.hosts {
		if h.latencyMs == 0 && h.Err == "" {
			return false
		}
	}
	return true
}

func (m model) maxLatency() int64 {
	var max int64
	for _, h := range m.hosts {
		if h.latencyMs > max {
			max = h.latencyMs
		}
	}
	return max
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
		case tea.MouseMsg:
			// Click on a tab to switch to it.
			if msg.Type == tea.MouseLeft && msg.Y <= 1 {
				x := msg.X
				// Rendered layout (measured): " name " with Padding(0,1) plus
				// leading space = 2 left, 1 right; 3-char gap between tabs.
				offset := 0
				for i, t := range tabNames() {
					tabW := len(t) + 3
					if x >= offset && x < offset+tabW {
						m.activeTab = i
						return m, nil
					}
					offset += tabW + 3
				}
			}
			return m, nil
	case tea.KeyMsg:
		// Scratch input mode (tab 7): capture typing.
		if m.activeTab == 7 {
			switch msg.String() {
			case "enter":
				if strings.TrimSpace(m.scratchBuf) != "" {
					m.scratch = append(m.scratch, m.scratchBuf)
					_ = appendScratch(m.scratchBuf)
					m.scratchBuf = ""
				}
			case "esc":
				m.scratchBuf = ""
			case "backspace":
				if len(m.scratchBuf) > 0 {
					m.scratchBuf = m.scratchBuf[:len(m.scratchBuf)-1]
				}
			case "q":
				if m.scratchBuf == "" {
					return m, tea.Quit
				}
			default:
				// Single printable char → append to buffer (but digits 1-8
				// switch tabs; only letters/spaces/other go to the buffer).
				if len(msg.String()) == 1 && !strings.ContainsAny(msg.String(), "12345678") {
					m.scratchBuf += msg.String()
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "q", "ctrl+c":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % 8
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab + 7) % 8
		case "1":
			m.activeTab = 0
		case "2":
			m.activeTab = 1
		case "3":
			m.activeTab = 2
		case "4":
			m.activeTab = 3
		case "5":
			m.activeTab = 4
		case "6":
			m.activeTab = 5
		case "7":
			m.activeTab = 6
		case "8":
			m.activeTab = 7
		}
		return m, nil
	case tickMsg:
		// Single-flight guard: skip this tick if a poll cycle is still running.
		if m.polling {
			return m, nil
		}
		m.polling = true
		// Tab-aware gating: heavy collectors (k3s/mining/ai) only poll while
		// their tab is visible. Core host poll + spotify always run (cheap).
		batch := []tea.Cmd{pollAll(m.hosts), pollSpotifyCmd()}
		switch m.activeTab {
		case 3:
			batch = append(batch, pollK3sCmd())
		case 4:
			batch = append(batch, pollMiningCmd())
		case 5:
			batch = append(batch, pollAICmd())
		}
		return m, tea.Batch(batch...)
	case pollMsg:
		for i := range m.hosts {
			if m.hosts[i].Name == msg.host {
				// Compute net rate from delta vs previous cumulative bytes.
				if prev, ok := m.prevHosts[msg.host]; ok {
					dt := 2.0 // poll interval seconds
					dRx := msg.data.NetRX - prev.NetRX
					dTx := msg.data.NetTX - prev.NetTX
					if dRx >= 0 {
						msg.data.NetRXRate = dRx * 8 / 1e6 / dt
					}
					if dTx >= 0 {
						msg.data.NetTXRate = dTx * 8 / 1e6 / dt
					}
				}
				if m.prevHosts == nil {
					m.prevHosts = map[string]Host{}
				}
				m.prevHosts[msg.host] = msg.data
				// Append CPU sample to history (cap at 60 = 2 min).
				if m.hist == nil {
					m.hist = map[string][]float64{}
				}
				h := m.hist[msg.host]
				h = append(h, msg.data.CPU)
				if len(h) > 60 {
					h = h[len(h)-60:]
				}
				m.hist[msg.host] = h
				m.hosts[i] = msg.data
			}
		}
		// Mark cycle complete when all 4 hosts have updated this round.
		if m.cycleDone() {
			m.polling = false
		}
		return m, nil
	case k3sMsg:
		m.k3s = msg.state
		return m, nil
	case miningMsg:
		// miningMsg no longer used for GPU data — hosts carry it. Only
		// hashrate arrives here (polled at a slower cadence).
		m.mining = msg.state
		return m, nil
	case spotifyMsg:
		m.nowPlaying = msg.state
		return m, nil
	case aiMsg:
		m.ai = msg.state
		return m, nil
	}
	return m, nil
}

// ── View ────────────────────────────────────────────────────────────────────

// tabNames is the single source of truth for tab order (render + click hit-test).
func tabNames() []string {
	return []string{"overview", "hosts", "builds", "k3s", "mining", "ai", "logs", "scratch"}
}

func (m model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	tabs := tabNames()
	tabLabels := make([]string, len(tabs))
	for i, t := range tabs {
		if i == m.activeTab {
			tabLabels[i] = lipgloss.NewStyle().Foreground(themeBg).Background(themeAccent).Bold(true).Padding(0, 1).Render(" " + t + " ")
		} else {
			tabLabels[i] = lipgloss.NewStyle().Foreground(themeDim).Padding(0, 1).Render(" " + t + " ")
		}
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, tabLabels...)
	header += "\n" + lipgloss.NewStyle().Foreground(themeDim).Render(strings.Repeat("─", m.width)) + "\n"

	var body string
	switch m.activeTab {
	case 0:
		body = m.renderOverview()
	case 1:
		body = m.renderHosts()
	case 2:
		body = m.renderBuilds()
	case 3:
		body = m.renderK3s()
	case 4:
		body = m.renderMining()
	case 5:
		body = m.renderAI()
	case 6:
		body = m.renderLogs()
	case 7:
		body = m.renderScratch()
	}

	// Now-playing strip (when available) — always visible at the bottom.
	np := m.renderNowPlaying()
	lat := m.maxLatency()
	footer := "\n" + np + statusStyle.Render(fmt.Sprintf("q quit · Tab switch · 1-8 jump · ? help · poll %dms", lat))

	if m.showHelp {
		body = m.renderHelp()
	}

	return lipgloss.NewStyle().Background(themeBg).Render(header + body + footer)
}

func (m model) renderHelp() string {
	help := lipgloss.NewStyle().
		Width(70).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bgSel).
		Padding(1, 2).
		Render(
			styleEmph.Render("fleet-deck — keys") + "\n\n" +
				"  " + styleAccent.Render("Tab") + " / " + styleAccent.Render("Shift+Tab") + "   switch panel\n" +
				"  " + styleAccent.Render("1-8") + "              jump to tab\n" +
				"  " + styleAccent.Render("?") + "                help overlay\n" +
				"  " + styleAccent.Render("q") + " / " + styleAccent.Render("Ctrl+C") + "        quit\n\n" +
				styleMuted.Render("click a tab to switch · 2s refresh · all data read-only"),
		)
	return help
}

func fmtNet(mbps float64) string {
	if mbps >= 1 {
		return fmt.Sprintf("%.1f Mbps", mbps)
	}
	return fmt.Sprintf("%.0f K/s", mbps*1000)
}

func (m model) renderOverview() string {
	// btop-style: colored host name + braille CPU graph + gauges per host.
	var rows []string
	for _, h := range m.hosts {
		status := h.Err
		if status == "" {
			status = "ok"
		}
		memPct := 0.0
		if h.MemTotal > 0 {
			memPct = h.MemUsed / h.MemTotal * 100
		}
		// Braille CPU sparkline (btop signature) — 8 cells to fit one line.
		cpuLine := ""
		if hist := m.hist[h.Name]; len(hist) > 0 {
			cpuLine = styleInfo.Render(brailleLine(hist, 8)) + " "
		}
		// Colored host name + braille graph + all metrics on ONE line (btop density).
		nameStyle := styleAccent.Render(h.Name)
		// Truncate long error text to keep the row one line.
		statusText := status
		if strings.Contains(statusText, "\n") {
			statusText = strings.SplitN(statusText, "\n", 2)[0]
		}
		if len(statusText) > 30 {
			statusText = statusText[:30] + "…"
		}
		row := fmt.Sprintf(" %s %s cpu%s%5.1f%% mem%s%5.1f/%5.1fG net↓%s↑%s disk%5.1f/%5.1fG %s",
			nameStyle, cpuLine, btopBar(h.CPU), h.CPU, btopBar(memPct), h.MemUsed, h.MemTotal, fmtNet(h.NetRXRate), fmtNet(h.NetTXRate), h.DiskUsed, h.DiskTotal, statusText)
		rows = append(rows, row)
	}
	return panelTitle("fleet overview").Render("fleet overview") + "\n" +
		overviewStyle.Render(strings.Join(rows, "\n"))
}

func (m model) renderHosts() string {
	// hosts tab: per-host detail (GPU, temps, builds)
	var parts []string
	for _, h := range m.hosts {
		title := panelTitle(h.Name).Render(h.Name)
		var lines []string
		lines = append(lines, fmt.Sprintf("cpu: %5.1f%%  temp: %5.1f°C", h.CPU, h.Temp))
		lines = append(lines, fmt.Sprintf("mem: %6.1f / %6.1f GB", h.MemUsed, h.MemTotal))
		lines = append(lines, fmt.Sprintf("net: ↓%s  ↑%s", fmtNet(h.NetRXRate), fmtNet(h.NetTXRate)))
		lines = append(lines, fmt.Sprintf("disk: %5.1f / %5.1f GB", h.DiskUsed, h.DiskTotal))
		for _, g := range h.GPU {
			lines = append(lines, fmt.Sprintf("gpu%d %s: %5.1f%% util %5.1f°C %6.1fW vram %4.1fGB", g.Index, g.Name, g.Util, g.Temp, g.Power, g.Mem))
		}
		if h.Err != "" {
			lines = append(lines, "err: "+h.Err)
		}
		parts = append(parts, panelStyle.Render(title+"\n"+strings.Join(lines, "\n")))
	}
	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

func (m model) renderBuilds() string {
	var rows []string
	for _, h := range m.hosts {
		if len(h.Builds) == 0 {
			rows = append(rows, fmt.Sprintf(" %-8s (no builds)", h.Name))
			continue
		}
		for _, b := range h.Builds {
			rows = append(rows, fmt.Sprintf(" %-8s %s", h.Name, b))
		}
	}
	return panelTitle("builds").Render("builds") + "\n" +
		buildsStyle.Render(strings.Join(rows, "\n"))
}

func (m model) renderLogs() string {
	// Placeholder: remote log tail (populated in logs.go / later)
	return panelStyle.Render(panelTitle("logs").Render("logs") + "\n" + " (log tail coming — wired to nexus build log)")
}

func (m model) renderK3s() string {
	var b strings.Builder
	b.WriteString(panelTitle("k3s cluster").Render("k3s cluster") + "\n")
	if m.k3s.Err != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(themeRed).Render(" " + m.k3s.Err) + "\n")
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Render(" NODES") + "\n")
		for _, n := range m.k3s.Nodes {
			b.WriteString("  " + n + "\n")
		}
		b.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Render(" PODS") + "\n")
		for _, p := range m.k3s.Pods {
			b.WriteString("  " + p + "\n")
		}
	}
	return panelStyle.Render(b.String())
}

func (m model) renderMining() string {
	var b strings.Builder
	b.WriteString(panelTitle("mining fleet").Render("mining fleet") + "\n")
	// GPU data comes from the live host poll (m.hosts); hashrate from m.mining.
	shown := false
	for _, h := range m.hosts {
		if !h.HasGPU || h.Err != "" {
			continue
		}
		shown = true
		b.WriteString(lipgloss.NewStyle().Foreground(themeYellow).Bold(true).Render(" "+h.Name) + "\n")
		for _, g := range h.GPU {
			line := fmt.Sprintf("  gpu%d %-20s util %5.1f%%  temp %5.1f°C  pow %6.1fW  vram %5.1fGB",
				g.Index, g.Name, g.Util, g.Temp, g.Power, g.Mem)
			b.WriteString(line + "\n")
		}
		// Hashrate from the separate slow-cadence mining poll.
		for _, mh := range m.mining.Hosts {
			if mh.Name == h.Name && mh.TotalHash > 0 {
				b.WriteString(fmt.Sprintf("  ⛏ hashrate: %.1f %s\n", mh.TotalHash, mh.HashUnit))
			}
		}
	}
	if !shown {
		b.WriteString(" (no GPU hosts online)\n")
	}
	return panelStyle.Render(b.String())
}

func (m model) renderAI() string {
	var b strings.Builder
	b.WriteString(panelTitle("ai inference").Render("ai inference") + "\n")
	if len(m.ai.Servers) == 0 && len(m.ai.Procs) == 0 {
		b.WriteString(" (no inference servers detected)\n")
	}
	if len(m.ai.Servers) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Render(" SERVERS") + "\n")
		for _, s := range m.ai.Servers {
			b.WriteString(fmt.Sprintf("  %-8s %-12s %-16s %s\n", s.Host, s.Name, s.Model, s.State))
		}
	}
	if len(m.ai.Procs) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(themeCyan).Render(" PROCESSES") + "\n")
		for _, p := range m.ai.Procs {
			b.WriteString("  " + p + "\n")
		}
	}
	return panelStyle.Render(b.String())
}

func (m model) renderScratch() string {
	var b strings.Builder
	b.WriteString(panelTitle("thoughts scratchboard").Render("thoughts scratchboard") + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(themeDim).Render(" (type a thought + Enter to save · esc to clear input · persistent at ~/.fleet-deck/thoughts.md)") + "\n")
	if len(m.scratch) == 0 {
		b.WriteString(" (empty)\n")
	}
	for i, l := range m.scratch {
		b.WriteString(fmt.Sprintf(" %d. %s\n", i+1, l))
	}
	if m.scratchBuf != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(themeGreen).Render(" > " + m.scratchBuf) + "\n")
	}
	return panelStyle.Render(b.String())
}

func (m model) renderNowPlaying() string {
	if m.nowPlaying.Title == "" {
		return ""
	}
	state := "▶"
	if !m.nowPlaying.Playing {
		state = "⏸"
	}
	np := fmt.Sprintf("%s %s — %s", state, m.nowPlaying.Title, m.nowPlaying.Artist)
	if m.nowPlaying.Progress != "" {
		np += "  [" + m.nowPlaying.Progress + "]"
	}
	return lipgloss.NewStyle().Foreground(themeGreen).Render(np) + "\n"
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "fleet-deck:", err)
		os.Exit(1)
	}
}
