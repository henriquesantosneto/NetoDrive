package syncer

import "testing"

func TestPlanDirSyncNewLocal(t *testing.T) {
	p := planDirSync(
		map[string]bool{"docs": true},
		map[string]bool{},
		map[string]bool{},
	)
	if len(p.upload) != 1 || p.upload[0] != "docs" {
		t.Fatalf("got %#v", p)
	}
}

func TestPlanDirSyncRemoteDelete(t *testing.T) {
	p := planDirSync(
		map[string]bool{},
		map[string]bool{"old": true},
		map[string]bool{"old": true},
	)
	if len(p.deleteRemote) != 1 || p.deleteRemote[0] != "old" {
		t.Fatalf("got %#v", p)
	}
}

func TestDirsChanged(t *testing.T) {
	if !dirsChanged(map[string]bool{"a": true}, map[string]bool{}) {
		t.Fatal("expected changed")
	}
	if dirsChanged(map[string]bool{"a": true}, map[string]bool{"a": true}) {
		t.Fatal("expected unchanged")
	}
}
