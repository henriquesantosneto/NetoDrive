package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlaceholderQueueEnqueue(t *testing.T) {
	syncRoot := t.TempDir()
	meta := placeholderMeta{Hash: "deadbeef", Size: 42}
	if err := enqueuePlaceholder(syncRoot, "rufus-4.15p.exe", meta); err != nil {
		t.Fatal(err)
	}
	if !isPlaceholderQueued(syncRoot, "rufus-4.15p.exe", meta.Hash) {
		t.Fatal("expected queued entry")
	}
	path := placeholderQueuePath(syncRoot)
	if stringsHasPrefix(path, syncRoot) {
		t.Fatalf("queue must live outside sync root: %s", path)
	}
	if err := enqueuePlaceholder(syncRoot, "rufus-4.15p.exe", meta); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if count := countLines(string(b)); count != 1 {
		t.Fatalf("expected 1 queue line, got %d", count)
	}
}

func TestPlaceholderUpToDateQueued(t *testing.T) {
	dir := t.TempDir()
	meta := placeholderMeta{Hash: "abc", Size: 10}
	if err := writePlaceholderMeta(dir, "a.txt", meta); err != nil {
		t.Fatal(err)
	}
	if placeholderUpToDate(dir, "a.txt", meta) {
		t.Fatal("meta alone should not be up to date without provider")
	}
	if err := enqueuePlaceholder(dir, "a.txt", meta); err != nil {
		t.Fatal(err)
	}
	if !placeholderUpToDate(dir, "a.txt", meta) {
		t.Fatal("queued placeholder should be up to date")
	}
}

func TestPlaceholderQueueRemoveRel(t *testing.T) {
	syncRoot := t.TempDir()
	meta := placeholderMeta{Hash: "abc", Size: 1}
	if err := enqueuePlaceholder(syncRoot, "a.txt", meta); err != nil {
		t.Fatal(err)
	}
	if err := removePlaceholderQueueRel(syncRoot, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if isPlaceholderQueuedRel(syncRoot, "a.txt") {
		t.Fatal("queue entry should be removed")
	}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func countLines(s string) int {
	n := 0
	for _, line := range splitLines(s) {
		if line != "" {
			n++
		}
	}
	return n
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func TestIsPlaceholderRelMetaOnly(t *testing.T) {
	dir := t.TempDir()
	if IsPlaceholderRel(dir, "x.bin") {
		t.Fatal("missing meta")
	}
	if err := writePlaceholderMeta(dir, "x.bin", placeholderMeta{Hash: "h", Size: 1}); err != nil {
		t.Fatal(err)
	}
	if !IsPlaceholderRel(dir, "x.bin") {
		t.Fatal("meta sidecar should count as placeholder")
	}
	_ = filepath.Join(dir, "x.bin")
}
