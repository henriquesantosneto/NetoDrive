package syncer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetaStoreOutsideSyncRoot(t *testing.T) {
	syncRoot := t.TempDir()
	metaPath := metaSidecarPath(syncRoot, "docs/a.txt")
	if strings.HasPrefix(metaPath, syncRoot+string(os.PathSeparator)) {
		t.Fatalf("meta sidecar must not live inside sync root: %s", metaPath)
	}
	if err := writePlaceholderMeta(syncRoot, "docs/a.txt", placeholderMeta{Hash: "abc", Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("meta not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(syncRoot, ".netodrive")); !os.IsNotExist(err) {
		t.Fatal("should not create .netodrive inside sync root")
	}
}
