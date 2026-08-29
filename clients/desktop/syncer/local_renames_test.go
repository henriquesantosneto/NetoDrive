package syncer

import "testing"

func TestEnqueueLocalRenameDedupes(t *testing.T) {
	root := t.TempDir()
	if err := EnqueueLocalRename(root, "a.txt", "b.txt"); err != nil {
		t.Fatal(err)
	}
	if err := EnqueueLocalRename(root, "a.txt", "b.txt"); err != nil {
		t.Fatal(err)
	}
	set, err := PendingLocalRenameSet(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(set) != 1 {
		t.Fatalf("expected 1 rename, got %d", len(set))
	}
}

func TestMigratePinnedPathsRename(t *testing.T) {
	pinned := []string{"docs", "photos/a.jpg"}
	got := migratePinnedPaths(pinned, "photos/a.jpg", "photos/b.jpg")
	if len(got) != 2 || got[0] != "docs" || got[1] != "photos/b.jpg" {
		t.Fatalf("unexpected pinned: %#v", got)
	}
}
