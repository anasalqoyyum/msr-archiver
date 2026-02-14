package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"msr-archiver/internal/api"
	"msr-archiver/internal/audio"
	"msr-archiver/internal/catalog"
	"msr-archiver/internal/config"
	"msr-archiver/internal/download"
	"msr-archiver/internal/logging"
	"msr-archiver/internal/metadata"
	"msr-archiver/internal/model"
	"msr-archiver/internal/state"
	"msr-archiver/internal/worker"
)

func main() {
	cfg := config.Parse()
	logger := logging.New()
	ctx := context.Background()

	if err := audio.CheckFFmpeg(ctx); err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		logger.Errorf("create output directory: %v", err)
		os.Exit(1)
	}

	store, err := state.NewStore(filepath.Join(cfg.OutputDir, "completed_albums.json"))
	if err != nil {
		logger.Errorf("initialize completion state: %v", err)
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	apiClient := api.New(httpClient)
	downloader := download.New(httpClient)
	albumCache := catalog.NewCache(resolveAlbumCachePath(cfg))

	albums, err := loadAlbums(ctx, cfg, logger, apiClient, albumCache)
	if err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}

	selectedAlbums, err := chooseAlbums(ctx, cfg, albums, store, apiClient)
	if err != nil {
		logger.Errorf("select albums: %v", err)
		os.Exit(1)
	}
	if len(selectedAlbums) == 0 {
		logger.Warnf("No albums selected; exiting")
		return
	}
	logger.Infof("Selected %d/%d albums for download", len(selectedAlbums), len(albums))

	jobs := make([]worker.Job, 0, len(selectedAlbums))
	for _, album := range selectedAlbums {
		album := album
		jobs = append(jobs, func(ctx context.Context) error {
			if err := processAlbum(ctx, cfg, logger, apiClient, downloader, store, album); err != nil {
				return fmt.Errorf("album %q: %w", album.Name, err)
			}
			return nil
		})
	}

	if err := worker.Run(ctx, cfg.Workers, jobs); err != nil {
		logger.Errorf("one or more albums failed: %v", err)
		os.Exit(1)
	}

	logger.Infof("All albums processed successfully")
}

func resolveAlbumCachePath(cfg config.Config) string {
	if strings.TrimSpace(cfg.AlbumCachePath) != "" {
		return cfg.AlbumCachePath
	}
	return filepath.Join(cfg.OutputDir, "albums_cache.json")
}

func loadAlbums(
	ctx context.Context,
	cfg config.Config,
	logger *logging.Logger,
	apiClient *api.Client,
	cache *catalog.Cache,
) ([]model.Album, error) {
	var cached []model.Album
	var cachedAt time.Time
	hasCached := false

	loaded, fetchedAt, err := cache.Load()
	if err == nil {
		cached = loaded
		cachedAt = fetchedAt
		hasCached = true
	} else if !os.IsNotExist(err) {
		logger.Warnf("Read album cache failed: %v", err)
	}

	if hasCached && !cfg.RefreshAlbums {
		if shouldUseCachedAlbums(cachedAt, cfg.AlbumCacheTTL, time.Now()) {
			logger.Infof("Loaded %d albums from cache: %s", len(cached), cache.Path())
			if !cachedAt.IsZero() {
				logger.Infof("Album cache timestamp: %s", cachedAt.Local().Format(time.RFC3339))
			}
			return cached, nil
		}

		if cfg.AlbumCacheTTL > 0 {
			if cachedAt.IsZero() {
				logger.Infof("Album cache has no timestamp; refreshing from API")
			} else {
				age := time.Since(cachedAt).Round(time.Second)
				logger.Infof("Album cache is stale (age %s > ttl %s); refreshing from API", age, cfg.AlbumCacheTTL)
			}
		}
	}

	logger.Infof("Fetching album catalog from API")
	albums, err := withRetryResult(ctx, 3, func() ([]model.Album, error) {
		return apiClient.GetAlbums(ctx)
	})
	if err != nil {
		if hasCached {
			logger.Warnf("Fetch albums failed (%v); using cached catalog with %d albums", err, len(cached))
			return cached, nil
		}
		return nil, fmt.Errorf("fetch albums: %w", err)
	}

	logger.Infof("Fetched %d albums from API", len(albums))
	if err := cache.Save(albums); err != nil {
		logger.Warnf("Persist album cache failed: %v", err)
	} else {
		logger.Infof("Updated album cache: %s", cache.Path())
	}

	return albums, nil
}

func shouldUseCachedAlbums(cachedAt time.Time, ttl time.Duration, now time.Time) bool {
	if ttl <= 0 {
		return true
	}
	if cachedAt.IsZero() {
		return false
	}

	age := now.Sub(cachedAt)
	if age < 0 {
		return true
	}
	return age <= ttl
}

func chooseAlbums(
	ctx context.Context,
	cfg config.Config,
	albums []model.Album,
	store *state.Store,
	apiClient *api.Client,
) ([]model.Album, error) {
	if len(albums) == 0 {
		return nil, nil
	}

	if strings.TrimSpace(cfg.Albums) != "" {
		return selectAlbumsByQuery(albums, cfg.Albums)
	}
	if cfg.ChooseAlbums {
		return chooseAlbumsInteractively(ctx, albums, store, apiClient)
	}
	return albums, nil
}

func selectAlbumsByQuery(albums []model.Album, raw string) ([]model.Album, error) {
	parts := strings.Split(raw, ",")
	queries := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			queries = append(queries, p)
		}
	}
	if len(queries) == 0 {
		return nil, fmt.Errorf("no valid album query provided")
	}

	seen := make(map[string]struct{})
	selected := make([]model.Album, 0, len(queries))
	for _, q := range queries {
		if strings.EqualFold(q, "all") {
			return albums, nil
		}

		match, err := resolveAlbumQuery(albums, q)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[match.CID]; ok {
			continue
		}
		seen[match.CID] = struct{}{}
		selected = append(selected, match)
	}

	return selected, nil
}

func resolveAlbumQuery(albums []model.Album, query string) (model.Album, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return model.Album{}, fmt.Errorf("empty album query")
	}

	exact := make([]model.Album, 0, 1)
	for _, a := range albums {
		if strings.EqualFold(a.CID, q) || strings.EqualFold(a.Name, q) {
			exact = append(exact, a)
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return model.Album{}, fmt.Errorf("album query %q matched multiple albums exactly; use CID", query)
	}

	contains := make([]model.Album, 0, 4)
	for _, a := range albums {
		if strings.Contains(strings.ToLower(a.Name), q) || strings.Contains(strings.ToLower(a.CID), q) {
			contains = append(contains, a)
		}
	}
	if len(contains) == 1 {
		return contains[0], nil
	}
	if len(contains) > 1 {
		labels := make([]string, 0, len(contains))
		for _, a := range contains {
			labels = append(labels, fmt.Sprintf("%s (%s)", a.Name, a.CID))
		}
		sort.Strings(labels)
		return model.Album{}, fmt.Errorf("album query %q is ambiguous: %s", query, strings.Join(labels, ", "))
	}

	return model.Album{}, fmt.Errorf("album query %q not found", query)
}

func albumOptionLabel(order int, album model.Album, completed bool) string {
	if completed {
		return fmt.Sprintf("%3d. %s (%s) [downloaded]", order, album.Name, album.CID)
	}
	return fmt.Sprintf("%3d. %s (%s)", order, album.Name, album.CID)
}

func selectedIndexesFromSet(selected map[int]struct{}) []int {
	indexes := make([]int, 0, len(selected))
	for idx := range selected {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	return indexes
}

func buildSelectedAlbumsPreview(albums []model.Album, selectedIndexes []int, maxItems int) string {
	if len(selectedIndexes) == 0 {
		return "No albums selected yet."
	}
	if maxItems < 1 {
		maxItems = 1
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Selected %d album(s):", len(selectedIndexes)))

	shown := 0
	for _, idx := range selectedIndexes {
		if idx < 0 || idx >= len(albums) {
			continue
		}
		shown++
		album := albums[idx]
		b.WriteString(fmt.Sprintf("\n%d. %s (%s)", shown, album.Name, album.CID))
		if shown >= maxItems {
			break
		}
	}

	if len(selectedIndexes) > shown {
		b.WriteString(fmt.Sprintf("\n... and %d more", len(selectedIndexes)-shown))
	}

	return b.String()
}

func albumsFromIndexes(albums []model.Album, indexes []int) ([]model.Album, error) {
	if len(indexes) == 0 {
		return nil, fmt.Errorf("no albums selected")
	}

	seen := make(map[int]struct{}, len(indexes))
	selected := make([]model.Album, 0, len(indexes))
	for _, idx := range indexes {
		if idx < 0 || idx >= len(albums) {
			return nil, fmt.Errorf("selected album index %d out of bounds", idx)
		}
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		selected = append(selected, albums[idx])
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no albums selected")
	}
	return selected, nil
}

func processAlbum(
	ctx context.Context,
	cfg config.Config,
	logger *logging.Logger,
	apiClient *api.Client,
	downloader *download.Downloader,
	store *state.Store,
	album model.Album,
) error {
	if store.IsCompleted(album.Name) {
		logger.Infof("Skipping completed album: %s", album.Name)
		return nil
	}
	started := time.Now()
	logger.Infof("[%s] Starting album download", album.Name)

	albumDir := filepath.Join(cfg.OutputDir, download.MakeValid(album.Name))
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		return fmt.Errorf("create album directory: %w", err)
	}

	coverJPG := filepath.Join(albumDir, "cover.jpg")
	coverPNG := filepath.Join(albumDir, "cover.png")
	logger.Infof("[%s] Downloading album cover", album.Name)
	if err := withRetry(ctx, 3, func() error {
		_, err := downloader.DownloadToFile(ctx, album.CoverURL, coverJPG)
		return err
	}); err != nil {
		return fmt.Errorf("download album cover: %w", err)
	}

	if err := audio.ConvertToPNG(coverJPG, coverPNG); err != nil {
		return fmt.Errorf("convert cover to png: %w", err)
	}
	if err := os.Remove(coverJPG); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove source cover jpg: %w", err)
	}

	songs, err := withRetryResult(ctx, 3, func() ([]model.Song, error) {
		return apiClient.GetAlbumSongs(ctx, album.CID)
	})
	if err != nil {
		return fmt.Errorf("fetch album songs: %w", err)
	}
	totalSongs := len(songs)
	if totalSongs == 0 {
		logger.Warnf("[%s] Album has no songs; marking as completed", album.Name)
	}
	logger.Infof("[%s] Found %d songs", album.Name, totalSongs)

	for i, song := range songs {
		song := song
		track := i + 1
		logger.Infof("[%s] [%d/%d] Resolving track: %s", album.Name, track, totalSongs, song.Name)

		detail, err := withRetryResult(ctx, 3, func() (model.SongDetail, error) {
			return apiClient.GetSongDetail(ctx, song.CID)
		})
		if err != nil {
			return fmt.Errorf("fetch song detail for %q: %w", song.Name, err)
		}

		var lyricPath string
		if detail.LyricURL != "" {
			lyricPath = filepath.Join(albumDir, download.MakeValid(song.Name)+".lrc")
			if err := withRetry(ctx, 3, func() error {
				_, err := downloader.DownloadToFile(ctx, detail.LyricURL, lyricPath)
				return err
			}); err != nil {
				return fmt.Errorf("download lyric for %q: %w", song.Name, err)
			}
		}

		var songPath string
		var fileType string
		var dl download.FileDownloadResult
		progress := makeSongProgressLogger(logger, album.Name, song.Name, track, totalSongs)
		logger.Infof("[%s] [%d/%d] Downloading track: %s", album.Name, track, totalSongs, song.Name)
		if err := withRetry(ctx, 3, func() error {
			var dlErr error
			songPath, fileType, dl, dlErr = downloader.DownloadSongWithProgress(ctx, albumDir, song.Name, detail.SourceURL, progress)
			return dlErr
		}); err != nil {
			return fmt.Errorf("download song %q: %w", song.Name, err)
		}

		if err := metadata.Apply(ctx, metadata.Input{
			FilePath:     songPath,
			FileType:     fileType,
			Album:        album.Name,
			Title:        song.Name,
			AlbumArtists: album.Artistes,
			Artists:      song.Artistes,
			TrackNumber:  i + 1,
			CoverPath:    coverPNG,
			LyricPath:    lyricPath,
		}); err != nil {
			return fmt.Errorf("write metadata for %q: %w", song.Name, err)
		}

		logger.Infof(
			"[%s] [%d/%d] Finished track: %s (%s, %s, %s)",
			album.Name,
			track,
			totalSongs,
			song.Name,
			fileType,
			formatBytes(dl.BytesWritten),
			formatRate(dl.BytesWritten, dl.Duration),
		)
	}

	if err := store.MarkCompleted(album.Name); err != nil {
		return fmt.Errorf("persist completion state: %w", err)
	}

	logger.Infof("[%s] Completed album in %s", album.Name, time.Since(started).Round(time.Millisecond))
	return nil
}

func makeSongProgressLogger(
	logger *logging.Logger,
	albumName, songName string,
	track, totalTracks int,
) download.ProgressFunc {
	lastProgressBucket := int64(-1)
	var lastUnknownProgress time.Time

	return func(update download.ProgressUpdate) {
		if update.TotalBytes > 0 {
			progress := (update.BytesWritten * 100) / update.TotalBytes
			progressBucket := progress / 10
			if progress == 100 || progressBucket > lastProgressBucket {
				logger.Infof(
					"[%s] [%d/%d] Downloading %s: %d%% (%s/%s)",
					albumName,
					track,
					totalTracks,
					songName,
					progress,
					formatBytes(update.BytesWritten),
					formatBytes(update.TotalBytes),
				)
				lastProgressBucket = progressBucket
			}
			return
		}

		if update.BytesWritten == 0 {
			return
		}
		now := time.Now()
		if lastUnknownProgress.IsZero() || now.Sub(lastUnknownProgress) >= 2*time.Second {
			logger.Infof(
				"[%s] [%d/%d] Downloading %s: %s",
				albumName,
				track,
				totalTracks,
				songName,
				formatBytes(update.BytesWritten),
			)
			lastUnknownProgress = now
		}
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	value := float64(bytes)
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	idx := 0
	for value >= unit && idx < len(units)-1 {
		value /= unit
		idx++
	}
	return fmt.Sprintf("%.1f %s", value, units[idx])
}

func formatRate(bytes int64, duration time.Duration) string {
	if duration <= 0 {
		return "n/a"
	}
	bytesPerSecond := int64(float64(bytes) / duration.Seconds())
	return fmt.Sprintf("%s/s", formatBytes(bytesPerSecond))
}

func withRetry(ctx context.Context, attempts int, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}

	var err error
	for i := 1; i <= attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if i == attempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i) * 400 * time.Millisecond):
		}
	}
	return err
}

func withRetryResult[T any](ctx context.Context, attempts int, fn func() (T, error)) (T, error) {
	var zero T
	if attempts < 1 {
		attempts = 1
	}

	var err error
	for i := 1; i <= attempts; i++ {
		value, callErr := fn()
		if callErr == nil {
			return value, nil
		}
		err = callErr
		if i == attempts {
			break
		}
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(time.Duration(i) * 400 * time.Millisecond):
		}
	}

	return zero, err
}
