package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Store manages completed album persistence.
type Store struct {
	path string

	mu  sync.Mutex
	set map[string]struct{}
}

// NewStore initializes state from completed_albums.json if present.
func NewStore(path string) (*Store, error) {
	s := &Store{
		path: path,
		set:  make(map[string]struct{}),
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read state file %s: %w", path, err)
	}

	var albums []string
	if err := json.Unmarshal(b, &albums); err != nil {
		return nil, fmt.Errorf("parse state file %s: %w", path, err)
	}

	for _, a := range albums {
		s.set[a] = struct{}{}
	}

	return s, nil
}

// IsCompleted reports whether an album has already been processed.
func (s *Store) IsCompleted(albumName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.set[albumName]
	return ok
}

// MarkCompleted records an album as completed and persists state atomically.
func (s *Store) MarkCompleted(albumName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.set[albumName]; ok {
		return nil
	}
	s.set[albumName] = struct{}{}

	albums := make([]string, 0, len(s.set))
	for name := range s.set {
		albums = append(albums, name)
	}
	sort.Strings(albums)

	payload, err := json.Marshal(albums)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state parent dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), filepath.Base(s.path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temporary state file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temporary state file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic replace state file: %w", err)
	}

	return nil
}
