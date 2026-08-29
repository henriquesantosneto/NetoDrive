package syncer

import "testing"

func TestPendingLocalDeleteSet(t *testing.T) {
	dir := t.TempDir()
	if err := EnqueueLocalDelete(dir, "docs/a.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := EnqueueLocalDelete(dir, "docs/a.pdf"); err != nil {
		t.Fatal(err)
	}
	set, err := PendingLocalDeleteSet(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !set["docs/a.pdf"] || len(set) != 1 {
		t.Fatalf("set: %v", set)
	}
	if !HasPendingLocalChanges(dir) {
		t.Fatal("expected pending")
	}
	if err := ClearLocalDelete(dir, "docs/a.pdf"); err != nil {
		t.Fatal(err)
	}
	if HasPendingLocalChanges(dir) {
		t.Fatal("expected cleared")
	}
}

func TestFilterKnownExcludingDeletes(t *testing.T) {
	dir := t.TempDir()
	known := map[string]string{"keep.txt": "h1", "gone.txt": "h2"}
	if err := EnqueueLocalDelete(dir, "gone.txt"); err != nil {
		t.Fatal(err)
	}
	got := filterKnownExcludingDeletes(dir, known)
	if len(got) != 1 || got["keep.txt"] != "h1" {
		t.Fatalf("got %v", got)
	}
}
