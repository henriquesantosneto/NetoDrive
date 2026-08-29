package syncer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTryQuickSyncSkipsWhenRemoteFileMissingLocally(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "sync-state.json")
	localRoot := filepath.Join(dir, "data")
	if err := os.MkdirAll(localRoot, 0o755); err != nil {
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
	ok, err := TryQuickSync(c, statePath, localRoot)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected full sync when remote file is not materialized locally")
	}
}
