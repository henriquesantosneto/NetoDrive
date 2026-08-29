package syncer

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenameFallsBackWhenAPIMissing(t *testing.T) {
	var uploaded, deleted string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/sync/manifest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"files":[{"path":"a.txt","hash":"h1","size":3,"is_dir":false}]}`))
		case r.URL.Path == "/api/sync/rename":
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/sync/download/"):
			_, _ = w.Write([]byte("abc"))
		case r.URL.Path == "/api/sync/upload" && r.Method == http.MethodPut:
			uploaded = r.Header.Get("X-File-Path")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"path":"abc.txt","hash":"h1","size":3}`))
		case r.URL.Path == "/api/files/a.txt" && r.Method == http.MethodDelete:
			deleted = "a.txt"
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Token: "t", HTTP: srv.Client()}
	if err := renameRemotePaths(c, "a.txt", "abc.txt", "", nil); err != nil {
		t.Fatal(err)
	}
	if uploaded != "abc.txt" {
		t.Fatalf("expected upload abc.txt, got %q", uploaded)
	}
	if deleted != "a.txt" {
		t.Fatalf("expected delete a.txt, got %q", deleted)
	}
}
