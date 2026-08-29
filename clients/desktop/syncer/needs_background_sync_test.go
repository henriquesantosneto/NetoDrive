package syncer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNeedsBackgroundSyncPendingPlaceholderQueue(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "sync-state.json")
	localRoot := filepath.Join(dir, "data")
	if err := os.MkdirAll(localRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	man := &Manifest{Version: 1, Files: []ManifestEntry{}}
	fp := manifestFingerprint(man)
	st := SyncState{LocalFolder: localRoot, LastManifestFP: fp, Known: map[string]string{}}
	if err := SaveState(statePath, st); err != nil {
		t.Fatal(err)
	}
	if err := enqueuePlaceholder(localRoot, "pending.exe", placeholderMeta{Hash: "abc", Size: 1}); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/sync/manifest":
			_ = json.NewEncoder(w).Encode(man)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev")
	need, err := NeedsBackgroundSync(c, statePath, localRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !need {
		t.Fatal("expected background sync when placeholder queue has pending entries")
	}
}

func TestNeedsBackgroundSyncIdleWhenSynced(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "sync-state.json")
	localRoot := filepath.Join(dir, "data")
	if err := os.MkdirAll(localRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localRoot, "file.bin"), []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}

	man := &Manifest{
		Version: 1,
		Files: []ManifestEntry{
			{Path: "file.bin", Hash: "abc", Size: 10},
		},
	}
	fp := manifestFingerprint(man)
	st := SyncState{
		LocalFolder:    localRoot,
		LastManifestFP: fp,
		Known:          map[string]string{"file.bin": "abc"},
		KnownDirs:      map[string]bool{},
	}
	if err := SaveState(statePath, st); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/sync/manifest":
			_ = json.NewEncoder(w).Encode(man)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev")
	need, err := NeedsBackgroundSync(c, statePath, localRoot)
	if err != nil {
		t.Fatal(err)
	}
	if need {
		t.Fatal("expected no background sync when already in sync")
	}
}
