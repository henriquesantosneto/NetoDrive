package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoRootCachedFromState(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	isRepo := true
	st := SyncState{IsRepoRoot: &isRepo}
	setSyncWalkContext(repo, &st)
	if !syncWalkLocalRootIsRepo {
		t.Fatal("expected repo from state")
	}
	// Second call must not require Stat on sync root.
	setSyncWalkContext(repo, &st)
	if !syncWalkLocalRootIsRepo {
		t.Fatal("expected cached repo")
	}
}
