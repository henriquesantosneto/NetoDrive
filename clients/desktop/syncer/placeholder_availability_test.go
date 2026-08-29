package syncer

import (
	"os"
	"testing"
)

func TestCloudOnlyPlaceholderMeta(t *testing.T) {
	root := t.TempDir()
	cloud := placeholderMeta{Hash: "h1", Size: 2, CloudOnly: boolPtr(true)}
	if err := writePlaceholderMeta(root, "a.txt", cloud); err != nil {
		t.Fatal(err)
	}
	if !isCloudOnlyPlaceholder(root, "a.txt") {
		t.Fatal("expected cloud-only")
	}

	local := placeholderMeta{Hash: "h1", Size: 2, CloudOnly: boolPtr(false)}
	if err := writePlaceholderMeta(root, "b.txt", local); err != nil {
		t.Fatal(err)
	}
	if isCloudOnlyPlaceholder(root, "b.txt") {
		t.Fatal("hydrated meta should not be cloud-only")
	}
}

func TestLocalHashForSyncHydratedUsesContent(t *testing.T) {
	root := t.TempDir()
	path := placeholderPath(root, "edited.txt")
	if err := writeHydratedMeta(root, "edited.txt", placeholderMeta{Hash: "old", Size: 3}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new-content-longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := localHashForSync(root, "edited.txt")
	if err != nil {
		t.Fatal(err)
	}
	if h == "old" {
		t.Fatal("expected content hash, not sidecar hash")
	}
}
