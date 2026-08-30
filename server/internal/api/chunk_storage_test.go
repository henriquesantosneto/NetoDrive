package api_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/netodrive/server/internal/api"
	"github.com/netodrive/server/internal/auth"
	"github.com/netodrive/server/internal/config"
	"github.com/netodrive/server/internal/storage"
	"github.com/netodrive/server/internal/store"
)

func setupChunkStorage(t *testing.T) (*httptest.Server, *storage.Service) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        dir,
		DBPath:         filepath.Join(dir, "test.db"),
		JWTSecret:      "test-secret",
		AdminUser:      "admin",
		AdminPass:      "admin123",
		MaxUploadBytes: 32 << 20,
		ChunkStorage:   true,
	}
	st, err := store.Open(cfg.DBPath, cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	hash, err := auth.HashPassword(cfg.AdminPass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(cfg.AdminUser, hash); err != nil {
		t.Fatal(err)
	}
	chunkRoot := filepath.Join(dir, "chunk-storage")
	chunkSvc, err := storage.Open(storage.Config{RootDir: chunkRoot})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = chunkSvc.Close() })
	srv := api.New(cfg, st, auth.New(cfg.JWTSecret))
	srv.ChunkStorage = chunkSvc
	srv.AttachChunkPurgeHook()
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return ts, chunkSvc
}

func TestUploadDownloadWithChunkStorage(t *testing.T) {
	ts, _ := setupChunkStorage(t)
	token := login(t, ts.URL)
	content := bytes.Repeat([]byte("chunked-upload\n"), 5000)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/sync/upload", bytes.NewReader(content))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-File-Path", "docs/chunked.bin")
	req.Header.Set("X-File-Mime", "application/octet-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("upload %d: %s", res.StatusCode, b)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/sync/download/docs/chunked.bin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("download %d", res.StatusCode)
	}
	if !bytes.Equal(got, content) {
		t.Fatal("download content mismatch")
	}
}
