package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/netodrive/server/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	hash, err := st.CreateUser("admin", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_ = hash
	return st
}

func upsertFile(t *testing.T, st *store.Store, userID int64, path string, isDir bool) {
	t.Helper()
	meta := &store.FileMeta{
		UserID: userID,
		Path:   path,
		Name:   filepath.Base(path),
		IsDir:  isDir,
		Mime:   "application/octet-stream",
		MTime:  time.Now().UTC(),
	}
	if isDir {
		meta.Mime = "inode/directory"
	}
	if err := st.UpsertFile(meta); err != nil {
		t.Fatal(err)
	}
}

func TestListDirShowsVirtualLegacyFolder(t *testing.T) {
	st := openTestStore(t)
	u, _ := st.GetUserByUsername("admin")

	upsertFile(t, st, u.ID, "PC/note.txt", false)

	files, err := st.ListDir(u.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range files {
		if f.Name == "PC" && f.IsDir {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected virtual PC folder at root, got %#v", files)
	}
}

func TestMigrateLegacyDevicePrefixes(t *testing.T) {
	st := openTestStore(t)
	u, _ := st.GetUserByUsername("admin")

	upsertFile(t, st, u.ID, "PC/doc.txt", false)

	n, err := st.MigrateLegacyDevicePrefixes()
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected moves, got %d", n)
	}

	f, err := st.GetFileByPath(u.ID, "doc.txt")
	if err != nil || f == nil {
		t.Fatal("doc.txt not migrated to root")
	}
	if _, err := st.GetFileByPath(u.ID, "PC/doc.txt"); err == nil {
		t.Fatal("legacy PC/doc.txt should be gone")
	}

	files, err := st.ListDir(u.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Name == "PC" {
			t.Fatalf("PC folder should not remain at root after migration: %#v", files)
		}
	}
}
