package store_test

import (
	"path/filepath"
	"testing"

	"github.com/netodrive/server/internal/store"
)

func TestRenamePathMovesRecordWithoutBlobChange(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	u, err := st.CreateUser("rename", "hash")
	if err != nil {
		t.Fatal(err)
	}
	meta := &store.FileMeta{
		UserID: u.ID,
		Path:   "old.txt",
		Name:   "old.txt",
		Hash:   "abc123",
		Size:   3,
		Mime:   "text/plain",
	}
	if err := st.UpsertFile(meta); err != nil {
		t.Fatal(err)
	}
	if err := st.RenamePath(u.ID, "old.txt", "new.txt"); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetFileByPath(u.ID, "new.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hash != "abc123" || got.Path != "new.txt" {
		t.Fatalf("unexpected renamed file: %+v", got)
	}
	if _, err := st.GetFileByPath(u.ID, "old.txt"); err == nil {
		t.Fatal("old path should be gone")
	}
}
