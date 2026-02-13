## msr-archiver

Download Arknights OSTs from `monster-siren.hypergryph.com` with album/song metadata, cover art, and lyrics.

## Behavior

- Downloads any albums and songs.
- Converts WAV sources to FLAC (`ffmpeg`).
- Writes metadata (`album`, `title`, `album artist`, `artist`, `track`).
- Embeds cover art and lyric metadata when available.
- Skips albums recorded in `completed_albums.json`.
- Caches fetched album catalog in `albums_cache.json` and refreshes automatically every 24 hours.
- Supports choosing specific albums (`--albums` or `--choose-albums`).
- Logs album/track progress with incremental download percentages and transfer rates.

## Requirements

- Go 1.25+
- `ffmpeg` available in `PATH`

## Run

```bash
go run ./cmd
```

Useful flags:

```bash
go run ./cmd --output ./MonsterSiren --workers 6 --http-timeout 2m --refresh-albums
go run ./cmd --albums "A Walk in the Dust,ab12cd34"
go run ./cmd --choose-albums=false
go run ./cmd --album-cache-ttl 24h
```

Album selection flags:
- `--albums`: comma-separated album names/CIDs (supports unique partial matches)
- `--choose-albums`: interactive numbered multi-select picker with built-in `/` filtering (shows `/input` while filtering), `[downloaded]` markers from `completed_albums.json`, and final selected-albums review (default: `true`)
- `--refresh-albums`: force refresh from API and update album cache
- `--album-cache`: custom cache file path
- `--album-cache-ttl`: cache max age before refresh (default `24h`, set `0` to disable TTL)

Build binary:

```bash
go build -o msr-archiver ./cmd
./msr-archiver --output ./MonsterSiren
```

## Tests

```bash
CGO_ENABLED=0 go test ./...
```
