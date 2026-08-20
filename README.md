# fleet-deck

Fleet observability dashboard — a Charm BubbleTea TUI for the NixOS cluster.
Replaces the btop/nvtop grid with one beautiful, interactive, Stylix-themed
view: hosts, builds, k3s, mining, AI inference, logs, and a thoughts
scratchboard.

## Run

```bash
go build -o fleet-deck . && ./fleet-deck
```

Read-only. Polls hosts over ssh (zephyr/nexus/sentry/forge). Never starts
builds or mutates the cluster.

## Keys

- `Tab` / `Shift+Tab` — switch panel
- `1-8` — jump to tab
- `?` — help overlay
- `q` / `Ctrl+C` — quit
- Click a tab to switch

## Tabs

1. **overview** — all 4 hosts: CPU braille graph + gauge, mem gauge, net rate, disk
2. **hosts** — per-host detail (CPU/temp/mem/net/disk/GPU)
3. **builds** — active nix/colmena builds (auto-discovered per host)
4. **k3s** — kubectl nodes + pods (via nexus)
5. **mining** — GPU util/temp/power + hashrate per mining host
6. **ai** — llama-swap endpoints + inference processes
7. **logs** — (wip)
8. **scratch** — persistent thoughts board (`~/.fleet-deck/thoughts.md`)

## Design

- Stylix "Osaka Jade" base16 palette (extracted from live alacritty config)
- Semantic color slots (tui-design): fg/bg/accent/status — never raw hex in widgets
- Widget-dashboard paradigm (btop-style): braille CPU graphs, threshold gauges
- Mouse-capable through tmux (click tabs), NO_COLOR-safe, 80x24 min
