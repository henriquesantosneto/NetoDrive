package store_test

import (
	"testing"
)

func TestSoftDeleteManyVirtualLegacyFolder(t *testing.T) {
	st := openTestStore(t)
	u, _ := st.GetUserByUsername("admin")

	upsertFile(t, st, u.ID, "PC/a.txt", false)
	upsertFile(t, st, u.ID, "PC/b.txt", false)

	n, err := st.SoftDeleteMany(u.ID, []string{"PC"})
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("expected deletes, got %d", n)
	}
	if _, err := st.GetFileByPath(u.ID, "PC/a.txt"); err == nil {
		f, _ := st.GetFileByPath(u.ID, "PC/a.txt")
		if f != nil && !f.Deleted {
			t.Fatal("PC/a.txt should be deleted")
		}
	}
}

func TestPurgeStaleLegacyPaths(t *testing.T) {
	st := openTestStore(t)
	u, _ := st.GetUserByUsername("admin")

	upsertFile(t, st, u.ID, "dup.txt", false)
	upsertFile(t, st, u.ID, "PC/dup.txt", false)

	n, err := st.MigrateLegacyDevicePrefixes()
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected migration activity, got %d", n)
	}
	if f, err := st.GetFileByPath(u.ID, "PC/dup.txt"); err == nil && f != nil && !f.Deleted {
		t.Fatal("stale PC/dup.txt should be removed when dup.txt exists")
	}
	if _, err := st.GetFileByPath(u.ID, "dup.txt"); err != nil {
		t.Fatal("dup.txt should remain at root")
	}
}
