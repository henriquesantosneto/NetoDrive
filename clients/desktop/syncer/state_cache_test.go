package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync-state.json")
	st := SyncState{
		LocalFolder:    dir,
		LastManifestFP: "abc123",
		Known:          map[string]string{"a.txt": "h1"},
	}
	if err := SaveStateCached(path, st); err != nil {
		t.Fatal(err)
	}
	got, err := LoadStateCached(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastManifestFP != "abc123" {
		t.Fatalf("fp: %q", got.LastManifestFP)
	}
	// Cached read should not touch disk (remove file).
	_ = os.Remove(path)
	got2, err := LoadStateCached(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got2.LastManifestFP != "abc123" {
		t.Fatal("expected memory cache hit")
	}
}
