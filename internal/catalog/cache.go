package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"msr-archiver/internal/model"
)

// Cache persists fetched album catalog data.
type Cache struct {
	path string
}

// NewCache creates an album catalog cache at a target path.
func NewCache(path string) *Cache {
	return &Cache{path: path}
}

type payload struct {
	FetchedAt string        `json:"fetchedAt"`
	Albums    []model.Album `json:"albums"`
}

// Load reads cached albums. If the file does not exist, os.ErrNotExist is returned.
func (c *Cache) Load() ([]model.Album, time.Time, error) {
	b, err := os.ReadFile(c.path)
	if err != nil {
		return nil, time.Time{}, err
	}

	var p payload
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, time.Time{}, fmt.Errorf("parse album cache %s: %w", c.path, err)
	}

	var fetchedAt time.Time
	if p.FetchedAt != "" {
		parsed, err := time.Parse(time.RFC3339, p.FetchedAt)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("parse fetchedAt in album cache %s: %w", c.path, err)
		}
		fetchedAt = parsed
	}

	return p.Albums, fetchedAt, nil
}

// Save writes album catalog data atomically.
func (c *Cache) Save(albums []model.Album) error {
	p := payload{
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Albums:    albums,
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal album cache: %w", err)
	}

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create album cache parent dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(c.path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temporary album cache file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temporary album cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temporary album cache file: %w", err)
	}

	if err := os.Rename(tmpPath, c.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic replace album cache file: %w", err)
	}
	return nil
}

// Path returns the cache file path.
func (c *Cache) Path() string {
	return c.path
}
