package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteDeletePathUsesLegacy(t *testing.T) {
	legacy := map[string]string{"docs/a.txt": "PC/docs/a.txt"}
	got := remoteDeletePath("docs/a.txt", "", legacy)
	if got != "PC/docs/a.txt" {
		t.Fatalf("got %q", got)
	}
	got = remoteDeletePath("hello.txt", "", nil)
	if got != "hello.txt" {
		t.Fatalf("got %q", got)
	}
}

func TestScanSkipsLegacyWhenRootCopyExists(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "note.txt")
	legacy := filepath.Join(dir, "PC", "note.txt")
	if err := os.WriteFile(root, []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := scanLocalFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Fatalf("expected one entry, got %#v", m)
	}
}
