package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanLocalFilesFindsUploadedPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := scanLocalFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m["hello.txt"] == "" {
		t.Fatalf("expected hello.txt in scan, got %#v", m)
	}
}
