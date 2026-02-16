## msr-archiver-tui

Bun + OpenTUI version of `msr-archiver` with interactive album selection, filtering, right-side song preview, and multi-worker downloads by default.

## Features

- Full download pipeline (albums, songs, lyrics, cover art, metadata embedding).
- Interactive album picker with:
  - `/` filter by album name or CID,
  - multi-select (`x` or `Space`),
  - `Ctrl+A` select/unselect all filtered,
  - live song preview pane on the right for the focused album,
  - copy-on-select to clipboard (OSC52) when selecting text with mouse.
- In-TUI download experience with:
  - confirmation screen before starting,
  - animated global and per-album progress bars,
  - live log panel,
  - completion summary screen.
- Album cache (`albums_cache.json`) with TTL logic.
- Completed album skip state (`completed_albums.json`).
- Concurrent album downloads (`--workers`, default `max(2, CPU)`).

## Requirements

- Bun
- `ffmpeg` in `PATH`

## Run

```bash
bun install
bun run src/index.tsx
```

Useful flags:

```bash
bun run src/index.tsx --output ./MonsterSiren --workers 6 --http-timeout 2m --refresh-albums
bun run src/index.tsx --albums "A Walk in the Dust,ab12cd34"
bun run src/index.tsx --choose-albums=false
bun run src/index.tsx --album-cache-ttl 24h
```

## Keybindings (picker)

- `j/k` or arrow keys: move
- `Ctrl+U` / `Ctrl+D`: page up/down
- `Space` or `x`: toggle current album
- `Ctrl+A`: toggle all currently filtered albums
- `/`: edit filter
- `Esc`: clear active filter
- `Enter`: open confirm dialog
- `Enter` / `s` / `y` (in confirm): start download
- `Esc` / `b` / `n` (in confirm): back to selection
- `i`: rerender inline cover (Kitty/Ghostty only)
- `Ctrl+C`: abort before downloads start
- `Enter` / `q` / `Esc` (after completion): exit TUI

## Album Art Rendering Feasibility

Album art now only renders through **Kitty graphics protocol** (Kitty/Ghostty).

- If Kitty protocol is available, inline cover rendering is auto-attempted and anchored at bottom-right.
- If Kitty protocol is not available, image rendering is disabled (no chafa fallback).

## tmux + Ghostty/Kitty passthrough

Like Yazi, kitty-protocol images inside tmux require passthrough support.

- The app now attempts `tmux set -p allow-passthrough on` automatically when running in tmux.
- You should also set this in your `tmux.conf` for persistence:

```tmux
set -g allow-passthrough on
set -ga update-environment TERM
set -ga update-environment TERM_PROGRAM
```

Then restart tmux server:

```bash
tmux kill-server && tmux || tmux
```
