package syncer

import "testing"

func TestDecidePendingRenameCancelWhenRemoteDeleted(t *testing.T) {
	man := &Manifest{Files: []ManifestEntry{}}
	got := decidePendingRename(man, "a.txt", "abc.txt", "", nil)
	if got != renameDecisionCancelRemoteDelete {
		t.Fatalf("expected cancel, got %d", got)
	}
}

func TestDecidePendingRenameApplyWhenSourceExists(t *testing.T) {
	man := &Manifest{Files: []ManifestEntry{{Path: "a.txt", Hash: "h", Size: 1}}}
	got := decidePendingRename(man, "a.txt", "abc.txt", "", nil)
	if got != renameDecisionApply {
		t.Fatalf("expected apply, got %d", got)
	}
}

func TestDecidePendingRenameDoneWhenTargetExists(t *testing.T) {
	man := &Manifest{Files: []ManifestEntry{{Path: "abc.txt", Hash: "h", Size: 1}}}
	got := decidePendingRename(man, "a.txt", "abc.txt", "", nil)
	if got != renameDecisionDone {
		t.Fatalf("expected done, got %d", got)
	}
}

func TestCancelPendingRenameClearsQueue(t *testing.T) {
	root := t.TempDir()
	statePath := root + "/state.json"
	st := SyncState{Known: map[string]string{"a.txt": "h1"}, Entries: map[string]FileEntry{}}
	_ = SaveState(statePath, st)

	if err := EnqueueLocalRename(root, "a.txt", "abc.txt"); err != nil {
		t.Fatal(err)
	}
	rn := localRename{From: "a.txt", To: "abc.txt"}
	if err := cancelPendingRename(root, &st, rn, "test"); err != nil {
		t.Fatal(err)
	}
	set, _ := PendingLocalRenameSet(root)
	if len(set) != 0 {
		t.Fatalf("rename queue should be empty, got %d", len(set))
	}
}
