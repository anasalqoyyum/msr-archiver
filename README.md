## msr-archiver

Download Arknights OSTs from `monster-siren.hypergryph.com` with album/song metadata, cover art, and lyrics.

## Behavior

- Downloads all albums and songs.
- Converts WAV sources to FLAC (`ffmpeg`).
- Writes metadata (`album`, `title`, `album artist`, `artist`, `track`).
- Embeds cover art and lyric metadata when available.
- Skips albums recorded in `completed_albums.json`.
- Caches fetched album catalog in `albums_cache.json`.
- Supports choosing specific albums (`--albums` or `--choose-albums`).
- Logs album/track progress with incremental download percentages and transfer rates.

## Requirements

- Go 1.22+
- `ffmpeg` available in `PATH`

## Run

```bash
go run ./cmd
```

Useful flags:

```bash
go run ./cmd --output ./MonsterSiren --workers 6 --http-timeout 2m --refresh-albums
go run ./cmd --albums "A Walk in the Dust,ab12cd34"
go run ./cmd --choose-albums
```

Album selection flags:
- `--albums`: comma-separated album names/CIDs (supports unique partial matches)
- `--choose-albums`: interactive index-based selection in terminal
- `--refresh-albums`: force refresh from API and update album cache
- `--album-cache`: custom cache file path

Build binary:

```bash
go build -o msr-archiver ./cmd
./msr-archiver --output ./MonsterSiren
```

## Tests

```bash
CGO_ENABLED=0 go test ./...
```

## Plan

Detailed implementation notes are in `MIGRATION_PLAN.md`.
