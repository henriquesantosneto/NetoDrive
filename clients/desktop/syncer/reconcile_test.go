package syncer

import "testing"

func TestPlanSyncWebDelete(t *testing.T) {
	local := map[string]string{"doc.txt": "aaa"}
	remote := map[string]string{}
	known := map[string]string{"doc.txt": "aaa"}

	p := planSync(local, remote, known, PlanSyncOptions{})
	if len(p.deleteLocal) != 1 || p.deleteLocal[0] != "doc.txt" {
		t.Fatalf("expected local delete, got %#v", p)
	}
	if len(p.upload) != 0 {
		t.Fatalf("should not re-upload deleted remote file: %#v", p)
	}
}

func TestPlanSyncLocalDelete(t *testing.T) {
	local := map[string]string{}
	remote := map[string]string{"doc.txt": "aaa"}
	known := map[string]string{"doc.txt": "aaa"}

	p := planSync(local, remote, known, PlanSyncOptions{})
	if len(p.deleteRemote) != 1 || p.deleteRemote[0] != "doc.txt" {
		t.Fatalf("expected remote delete, got %#v", p)
	}
	if len(p.download) != 0 {
		t.Fatalf("should not re-download locally deleted file: %#v", p)
	}
}

func TestPlanSyncNewLocalFile(t *testing.T) {
	local := map[string]string{"new.txt": "bbb"}
	remote := map[string]string{}
	known := map[string]string{}

	p := planSync(local, remote, known, PlanSyncOptions{})
	if len(p.upload) != 1 || p.upload[0] != "new.txt" {
		t.Fatalf("expected upload, got %#v", p)
	}
}

func TestPlanSyncNewRemoteFile(t *testing.T) {
	local := map[string]string{}
	remote := map[string]string{"remote.txt": "ccc"}
	known := map[string]string{}

	p := planSync(local, remote, known, PlanSyncOptions{})
	if len(p.download) != 1 || p.download[0] != "remote.txt" {
		t.Fatalf("expected download, got %#v", p)
	}
}

func TestPlanSyncCFAPIRematerializeWithMeta(t *testing.T) {
	root := t.TempDir()
	if err := writePlaceholderMeta(root, "doc.txt", placeholderMeta{Hash: "aaa", Size: 3}); err != nil {
		t.Fatal(err)
	}
	local := map[string]string{}
	remote := map[string]string{"doc.txt": "aaa"}
	known := map[string]string{"doc.txt": "aaa"}

	p := planSync(local, remote, known, PlanSyncOptions{
		LocalRoot:            root,
		RematerializeMissing: true,
	})
	if len(p.download) != 1 || p.download[0] != "doc.txt" {
		t.Fatalf("expected rematerialize download, got %#v", p)
	}
	if len(p.deleteRemote) != 0 {
		t.Fatalf("should not delete remote when meta pending: %#v", p)
	}
}

func TestPlanSyncCFAPIDeleteWhenKnownAbsentNoMeta(t *testing.T) {
	root := t.TempDir()
	local := map[string]string{}
	remote := map[string]string{"doc.txt": "aaa"}
	known := map[string]string{"doc.txt": "aaa"}

	p := planSync(local, remote, known, PlanSyncOptions{
		LocalRoot:            root,
		RematerializeMissing: true,
	})
	if len(p.deleteRemote) != 1 || p.deleteRemote[0] != "doc.txt" {
		t.Fatalf("expected remote delete without meta, got %#v", p)
	}
	if len(p.download) != 0 {
		t.Fatalf("should not rematerialize without meta sidecar: %#v", p)
	}
}

func TestPlanSyncCFAPIPendingDeleteStillDeletesRemote(t *testing.T) {
	root := t.TempDir()
	local := map[string]string{}
	remote := map[string]string{"doc.txt": "aaa"}
	known := map[string]string{"doc.txt": "aaa"}

	p := planSync(local, remote, known, PlanSyncOptions{
		LocalRoot:            root,
		RematerializeMissing: true,
		PendingLocalDeletes:  map[string]bool{"doc.txt": true},
	})
	if len(p.deleteRemote) != 1 || p.deleteRemote[0] != "doc.txt" {
		t.Fatalf("expected remote delete for pending local delete, got %#v", p)
	}
	if len(p.download) != 0 {
		t.Fatalf("should not download when user deleted locally: %#v", p)
	}
}
