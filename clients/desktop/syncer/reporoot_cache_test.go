package syncer

import "testing"

func TestNormalizeRootKeyCaseInsensitive(t *testing.T) {
	k1 := normalizeRootKey("/foo/Bar")
	k2 := normalizeRootKey("/foo/bar/")
	if k1 != k2 {
		t.Fatalf("keys differ: %q vs %q", k1, k2)
	}
	repoRootCache = map[string]bool{}
	repoRootCache[k1] = true
	st := SyncState{}
	setSyncWalkContext("/foo/Bar", &st)
	if !syncWalkLocalRootIsRepo {
		t.Fatal("expected cache hit across path casing")
	}
}

func TestPrepareRepoRootSkipsStatWhenCfapi(t *testing.T) {
	dir := t.TempDir()
	statePath := dir + "/sync-state.json"
	PrepareRepoRootCache(dir, statePath, nil)
	key := normalizeRootKey(dir)
	if _, ok := repoRootCache[key]; !ok {
		t.Fatal("expected cache entry")
	}
}
