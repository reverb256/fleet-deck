package main

import tea "github.com/charmbracelet/bubbletea"

// messages.go — Bubble Tea messages + poll commands for the extra panels.

type k3sMsg struct{ state K3sState }

func pollK3sCmd() tea.Cmd {
	return func() tea.Msg {
		return k3sMsg{state: collectK3s()}
	}
}

type miningMsg struct{ state MiningState }

func pollMiningCmd() tea.Cmd {
	return func() tea.Msg {
		return miningMsg{state: collectMining()}
	}
}

type spotifyMsg struct{ state NowPlaying }

func pollSpotifyCmd() tea.Cmd {
	return func() tea.Msg {
		return spotifyMsg{state: collectSpotify()}
	}
}

type aiMsg struct{ state AIState }

func pollAICmd() tea.Cmd {
	return func() tea.Msg {
		return aiMsg{state: collectAI()}
	}
}
