package syncer

import (
	"testing"
)

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
