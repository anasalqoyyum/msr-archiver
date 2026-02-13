package main

import (
	"testing"
	"time"
)

func TestShouldUseCachedAlbumsWithinTTL(t *testing.T) {
	now := time.Date(2026, time.February, 13, 10, 0, 0, 0, time.UTC)
	cachedAt := now.Add(-23 * time.Hour)

	if !shouldUseCachedAlbums(cachedAt, 24*time.Hour, now) {
		t.Fatalf("expected cache to be used when within TTL")
	}
}

func TestShouldUseCachedAlbumsStale(t *testing.T) {
	now := time.Date(2026, time.February, 13, 10, 0, 0, 0, time.UTC)
	cachedAt := now.Add(-25 * time.Hour)

	if shouldUseCachedAlbums(cachedAt, 24*time.Hour, now) {
		t.Fatalf("expected cache to be refreshed when stale")
	}
}

func TestShouldUseCachedAlbumsMissingTimestamp(t *testing.T) {
	now := time.Date(2026, time.February, 13, 10, 0, 0, 0, time.UTC)

	if shouldUseCachedAlbums(time.Time{}, 24*time.Hour, now) {
		t.Fatalf("expected cache without timestamp to be refreshed")
	}
}

func TestShouldUseCachedAlbumsDisabledTTL(t *testing.T) {
	now := time.Date(2026, time.February, 13, 10, 0, 0, 0, time.UTC)
	cachedAt := now.Add(-365 * 24 * time.Hour)

	if !shouldUseCachedAlbums(cachedAt, 0, now) {
		t.Fatalf("expected cache to be used when TTL is disabled")
	}
}
