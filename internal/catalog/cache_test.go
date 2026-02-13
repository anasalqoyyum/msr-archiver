package catalog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"msr-archiver/internal/model"
)

func TestCacheSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "albums_cache.json")
	cache := NewCache(cachePath)

	albums := []model.Album{
		{CID: "a1", Name: "Alpha", CoverURL: "https://cover/1", Artistes: []string{"Artist 1"}},
		{CID: "a2", Name: "Beta", CoverURL: "https://cover/2", Artistes: []string{"Artist 2"}},
	}

	if err := cache.Save(albums); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, fetchedAt, err := cache.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != len(albums) {
		t.Fatalf("expected %d albums, got %d", len(albums), len(loaded))
	}
	if loaded[0].CID != albums[0].CID || loaded[1].Name != albums[1].Name {
		t.Fatalf("loaded albums mismatch: %+v", loaded)
	}
	if fetchedAt.IsZero() {
		t.Fatalf("expected non-zero fetchedAt")
	}
}

func TestCacheLoadNotExist(t *testing.T) {
	cache := NewCache(filepath.Join(t.TempDir(), "missing.json"))
	_, _, err := cache.Load()
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestCacheLoadInvalidTimestamp(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "albums_cache.json")
	if err := os.WriteFile(cachePath, []byte(`{"fetchedAt":"not-a-time","albums":[]}`), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cache := NewCache(cachePath)
	_, _, err := cache.Load()
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestCacheSaveWritesRecentFetchedAt(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "albums_cache.json")
	cache := NewCache(cachePath)

	before := time.Now().Add(-2 * time.Second)
	if err := cache.Save(nil); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	_, fetchedAt, err := cache.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if fetchedAt.Before(before) {
		t.Fatalf("unexpected stale fetchedAt: %s", fetchedAt)
	}
}
