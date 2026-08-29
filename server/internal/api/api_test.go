package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/netodrive/server/internal/api"
	"github.com/netodrive/server/internal/auth"
	"github.com/netodrive/server/internal/config"
	"github.com/netodrive/server/internal/store"
)

func setup(t *testing.T) (*httptest.Server, string) {
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
	srv := api.New(cfg, st, auth.New(cfg.JWTSecret))
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return ts, ""
}

func login(t *testing.T, base string) string {
	t.Helper()
	body := `{"username":"admin","password":"admin123"}`
	res, err := http.Post(base+"/api/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("login status %d", res.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	token, _ := out["token"].(string)
	if token == "" {
		t.Fatal("empty token")
	}
	return token
}

func TestUploadDownloadOpenAndGallery(t *testing.T) {
	ts, _ := setup(t)
	token := login(t, ts.URL)

	content := []byte("hello netodrive " + time.Now().String())
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/sync/upload", bytes.NewReader(content))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-File-Path", "docs/hello.txt")
	req.Header.Set("X-File-Mime", "text/plain")
	req.Header.Set("X-Device-Id", "test")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("upload %d: %s", res.StatusCode, b)
	}

	// list root
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/files?path=", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("list %d: %s", res.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("docs")) {
		t.Fatalf("expected docs dir in %s", body)
	}

	// open remote
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/open/docs/hello.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("open %d", res.StatusCode)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch")
	}

	// gallery upload
	img := []byte{0xff, 0xd8, 0xff, 0xd9} // minimal jpeg-ish
	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/gallery/sync", bytes.NewReader(img))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Gallery-Key", "img-1")
	req.Header.Set("X-File-Path", "Galeria/Camera/img-1.jpg")
	req.Header.Set("X-Gallery-Album", "Camera")
	req.Header.Set("X-File-Mime", "image/jpeg")
	req.Header.Set("X-Device-Id", "android-test")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("gallery upload %d", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/gallery/albums", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(body, []byte("Camera")) {
		t.Fatalf("albums missing Camera: %s", body)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/gallery", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(body, []byte("img-1")) {
		t.Fatalf("gallery missing item: %s", body)
	}
}

func TestTrashRestoreAndPurge(t *testing.T) {
	ts, _ := setup(t)
	token := login(t, ts.URL)
	content := []byte("trash-me")
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/sync/upload", bytes.NewReader(content))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-File-Path", "tmp/delete-me.txt")
	req.Header.Set("X-Device-Id", "test")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/files/tmp/delete-me.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("soft delete %d", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/trash", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(body, []byte("delete-me.txt")) {
		t.Fatalf("trash missing file: %s", body)
	}

	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/trash/restore/tmp/delete-me.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("restore %d", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/files/tmp/delete-me.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ = http.DefaultClient.Do(req)
	res.Body.Close()

	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/trash/purge/tmp/delete-me.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("purge %d", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/trash", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ = http.DefaultClient.Do(req)
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if bytes.Contains(body, []byte("delete-me.txt")) {
		t.Fatalf("still in trash after purge: %s", body)
	}
}

func TestManifestSync(t *testing.T) {
	ts, _ := setup(t)
	token := login(t, ts.URL)
	payload := []byte("sync-me")
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/sync/upload", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-File-Path", "PC/a.txt")
	req.Header.Set("X-Device-Id", "pc")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/sync/manifest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(body, []byte("PC/a.txt")) {
		t.Fatalf("manifest: %s", body)
	}

	// download to temp via API path used by desktop client
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/sync/download/PC/a.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Equal(got, payload) {
		t.Fatalf("download mismatch")
	}
}

func TestSyncRename(t *testing.T) {
	ts, _ := setup(t)
	token := login(t, ts.URL)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/sync/upload", bytes.NewReader([]byte("data")))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-File-Path", "a.txt")
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	body, _ := json.Marshal(map[string]string{"from": "a.txt", "to": "abc.txt"})
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/sync/rename", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("rename status %d: %s", res.StatusCode, b)
	}
	res.Body.Close()

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/sync/manifest", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	manifest, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(manifest, []byte("abc.txt")) || bytes.Contains(manifest, []byte(`"path":"a.txt"`)) {
		t.Fatalf("manifest after rename: %s", manifest)
	}
}
