package config

import (
	"flag"
	"runtime"
	"time"
)

// Config contains runtime options for the downloader.
type Config struct {
	OutputDir      string
	Workers        int
	HTTPTimeout    time.Duration
	Albums         string
	ChooseAlbums   bool
	RefreshAlbums  bool
	AlbumCachePath string
}

// Parse reads CLI flags into Config.
func Parse() Config {
	defaultWorkers := runtime.NumCPU()
	if defaultWorkers < 2 {
		defaultWorkers = 2
	}

	outputDir := flag.String("output", "./MonsterSiren", "output directory")
	workers := flag.Int("workers", defaultWorkers, "number of concurrent album workers")
	httpTimeout := flag.Duration("http-timeout", 2*time.Minute, "HTTP request timeout")
	albums := flag.String("albums", "", "comma-separated album names or CIDs to download")
	chooseAlbums := flag.Bool("choose-albums", true, "interactively choose albums to download (default: true; set --choose-albums=false to download all)")
	refreshAlbums := flag.Bool("refresh-albums", false, "fetch album catalog from API and update cache")
	albumCachePath := flag.String("album-cache", "", "album cache file path (default: <output>/albums_cache.json)")

	flag.Parse()

	if *workers < 1 {
		*workers = 1
	}

	return Config{
		OutputDir:      *outputDir,
		Workers:        *workers,
		HTTPTimeout:    *httpTimeout,
		Albums:         *albums,
		ChooseAlbums:   *chooseAlbums,
		RefreshAlbums:  *refreshAlbums,
		AlbumCachePath: *albumCachePath,
	}
}
