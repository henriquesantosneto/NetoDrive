package syncer

import "testing"

func TestEnqueueProviderCommandDedupes(t *testing.T) {
	root := t.TempDir()
	if err := enqueueProviderCommand(root, "dehydrate", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := enqueueProviderCommand(root, "dehydrate", "a.txt"); err != nil {
		t.Fatal(err)
	}
	path := providerCommandsPath(root)
	b, err := readPlaceholderQueueLines(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1 {
		t.Fatalf("expected 1 command, got %d", len(b))
	}
}
