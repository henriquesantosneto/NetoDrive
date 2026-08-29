package syncer

import (
	"testing"
)

func TestScanCFAPIRootOnlyUsesMeta(t *testing.T) {
	root := t.TempDir()
	if err := writePlaceholderMeta(root, "cloud.bin", placeholderMeta{Hash: "abc", Size: 1}); err != nil {
		t.Fatal(err)
	}
	existing := indexMetaStore(root)
	found, err := scanCFAPIRootOnly(root, existing)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("meta-only files should not reappear in root scan: %v", found)
	}
}

func TestScanLocalFilesLightTrustsKnownWithMeta(t *testing.T) {
	root := t.TempDir()
	if err := writePlaceholderMeta(root, "a.txt", placeholderMeta{Hash: "h1", Size: 2}); err != nil {
		t.Fatal(err)
	}
	known := map[string]string{"a.txt": "h1"}
	local, err := scanLocalFilesLight(root, known)
	if err != nil {
		t.Fatal(err)
	}
	if local["a.txt"] != "h1" {
		t.Fatalf("meta index: got %q", local["a.txt"])
	}
}
