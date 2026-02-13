package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreMarkCompletedAndReload(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "completed_albums.json")

	store, err := NewStore(statePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if store.IsCompleted("alpha") {
		t.Fatalf("album unexpectedly marked as completed")
	}

	if err := store.MarkCompleted("alpha"); err != nil {
		t.Fatalf("MarkCompleted alpha failed: %v", err)
	}
	if err := store.MarkCompleted("beta"); err != nil {
		t.Fatalf("MarkCompleted beta failed: %v", err)
	}
	if err := store.MarkCompleted("alpha"); err != nil {
		t.Fatalf("MarkCompleted duplicate alpha failed: %v", err)
	}

	reloaded, err := NewStore(statePath)
	if err != nil {
		t.Fatalf("reload NewStore failed: %v", err)
	}

	if !reloaded.IsCompleted("alpha") || !reloaded.IsCompleted("beta") {
		t.Fatalf("reloaded store missing completed albums")
	}

	b, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var entries []string
	if err := json.Unmarshal(b, &entries); err != nil {
		t.Fatalf("state file should contain JSON array: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}
