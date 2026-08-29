package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteFilesNeedMaterializationMissingFile(t *testing.T) {
	root := t.TempDir()
	man := &Manifest{
		Files: []ManifestEntry{
			{Path: "web-only.txt", Hash: "abc", Size: 3},
		},
	}
	if !remoteFilesNeedMaterialization(root, man) {
		t.Fatal("expected materialization when remote file is absent locally")
	}
}

func TestRemoteFilesNeedMaterializationPresentFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "present.txt")
	if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	man := &Manifest{
		Files: []ManifestEntry{
			{Path: "present.txt", Hash: "abc", Size: 2},
		},
	}
	if remoteFilesNeedMaterialization(root, man) {
		t.Fatal("did not expect materialization when file exists locally")
	}
}

func TestIndexMetaStorePresentIgnoresSidecarWithoutFile(t *testing.T) {
	root := t.TempDir()
	if err := writePlaceholderMeta(root, "ghost.txt", placeholderMeta{Hash: "h1", Size: 2}); err != nil {
		t.Fatal(err)
	}
	all := indexMetaStore(root)
	if len(all) != 1 {
		t.Fatalf("meta sidecar expected, got %d", len(all))
	}
	present := indexMetaStorePresent(root)
	if len(present) != 0 {
		t.Fatalf("sidecar without disk file must not count as present, got %#v", present)
	}
}
