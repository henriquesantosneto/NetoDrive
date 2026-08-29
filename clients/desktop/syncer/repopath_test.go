package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsLikelyRepoRoot(t *testing.T) {
	dir := t.TempDir()
	if IsLikelyRepoRoot(dir) {
		t.Fatal("empty dir should not be repo")
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsLikelyRepoRoot(dir) {
		t.Fatal("expected repo root")
	}
}

func TestPlaceholderUpToDate(t *testing.T) {
	dir := t.TempDir()
	meta := placeholderMeta{Hash: "abc", Size: 10}
	if placeholderUpToDate(dir, "a.txt", meta) {
		t.Fatal("missing placeholder")
	}
	if err := writePlaceholderMagic(dir, "a.txt", meta); err != nil {
		t.Fatal(err)
	}
	if !placeholderUpToDate(dir, "a.txt", meta) {
		t.Fatal("expected up to date after magic write")
	}
}
