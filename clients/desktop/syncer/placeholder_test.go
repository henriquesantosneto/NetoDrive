package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlaceholderRoundTrip(t *testing.T) {
	dir := t.TempDir()
	meta := placeholderMeta{Hash: "abc123", Size: 42}
	if err := writePlaceholder(dir, "docs/a.txt", meta); err != nil {
		t.Fatal(err)
	}
	path := placeholderPath(dir, "docs/a.txt")
	if !IsPlaceholderFile(path) {
		t.Fatal("expected placeholder marker")
	}
	got, ok := readPlaceholderMeta(path)
	if !ok || got.Hash != meta.Hash || got.Size != meta.Size {
		t.Fatalf("meta mismatch %#v", got)
	}
	hash, isPh, err := hashForLocalPath(dir, "docs/a.txt")
	if err != nil || !isPh || hash != "abc123" {
		t.Fatalf("hashForLocalPath = %q ph=%v err=%v", hash, isPh, err)
	}
}

func TestIsPinnedPath(t *testing.T) {
	pins := []string{"docs", "Galeria/Camera"}
	if !isPinnedPath(pins, "docs/report.pdf") {
		t.Fatal("expected docs prefix pin")
	}
	if !isPinnedPath(pins, "Galeria/Camera/img.jpg") {
		t.Fatal("expected album pin")
	}
	if isPinnedPath(pins, "other.txt") {
		t.Fatal("should not pin unrelated path")
	}
}

func TestScanFindsPlaceholderHash(t *testing.T) {
	dir := t.TempDir()
	_ = writePlaceholder(dir, "x.txt", placeholderMeta{Hash: "deadbeef", Size: 1})
	m, err := scanLocalFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m["x.txt"] != "deadbeef" {
		t.Fatalf("scan = %#v", m)
	}
	if _, err := os.Stat(filepath.Join(dir, "x.txt")); err != nil {
		t.Fatal(err)
	}
}
