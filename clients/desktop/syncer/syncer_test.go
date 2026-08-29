package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/netodrive/desktop/syncer"
)

func TestFileHash(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	h1, n, err := syncer.FileHash(p)
	if err != nil || n != 3 || h1 == "" {
		t.Fatalf("hash=%s n=%d err=%v", h1, n, err)
	}
	h2, _, err := syncer.FileHash(p)
	if err != nil || h1 != h2 {
		t.Fatalf("unstable hash")
	}
}
