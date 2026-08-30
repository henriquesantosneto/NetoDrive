package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/netodrive/server/internal/storage"
)

func openTestStorage(t *testing.T) *storage.Service {
	t.Helper()
	dir := t.TempDir()
	s, err := storage.Open(storage.Config{RootDir: filepath.Join(dir, "storage")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestPutGetRoundTrip(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()
	data := bytes.Repeat([]byte("netodrive-chunk-storage-test\n"), 4000)
	man, err := s.Put(ctx, "test.bin", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if man.Size != int64(len(data)) {
		t.Fatalf("size mismatch: %d vs %d", man.Size, len(data))
	}
	rc, size, err := s.ReadFile(ctx, man.FileID)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(data)) || !bytes.Equal(got, data) {
		t.Fatal("content mismatch")
	}
}

func TestDedupSharesChunks(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()
	partA := bytes.Repeat([]byte("AAA"), 20000)
	partB := bytes.Repeat([]byte("BBB"), 20000)
	partC := bytes.Repeat([]byte("CCC"), 20000)
	partX := bytes.Repeat([]byte("XXX"), 20000)

	file1 := append(append([]byte{}, partA...), append(partB, partC...)...)
	file2 := append(append([]byte{}, partA...), append(partB, partX...)...)

	m1, err := s.Put(ctx, "f1", bytes.NewReader(file1))
	if err != nil {
		t.Fatal(err)
	}
	m2, err := s.Put(ctx, "f2", bytes.NewReader(file2))
	if err != nil {
		t.Fatal(err)
	}
	if len(m1.Chunks) == 0 || len(m2.Chunks) == 0 {
		t.Fatal("expected chunks")
	}

	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	chunks := stats["chunks"].(int64)
	if chunks >= int64(len(m1.Chunks)+len(m2.Chunks)) {
		// shared chunks must be fewer than sum of per-file chunk counts
		t.Fatalf("expected dedup to reduce chunk count, stats=%v m1=%d m2=%d", stats, len(m1.Chunks), len(m2.Chunks))
	}
}

func TestDeleteAndGC(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()
	data := bytes.Repeat([]byte("gc-test"), 8000)
	man, err := s.Put(ctx, "gc.bin", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFile(ctx, man.FileID); err != nil {
		t.Fatal(err)
	}
	if err := s.RunGC(ctx); err != nil {
		t.Fatal(err)
	}
	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats["chunks"].(int64) != 0 {
		t.Fatalf("expected no chunks after gc, got %v", stats)
	}
}

func TestManifestStored(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()
	data := bytes.Repeat([]byte("manifest"), 12000)
	man, err := s.Put(ctx, "m.bin", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetManifest(ctx, man.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FileID != man.FileID || got.Size != man.Size || len(got.Chunks) != len(man.Chunks) {
		t.Fatalf("manifest mismatch: %+v vs %+v", got, man)
	}
}

func TestConcurrentSameChunk(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()
	payload := bytes.Repeat([]byte("concurrent-chunk"), 12000)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("file-%d.bin", n)
			_, err := s.Put(ctx, name, bytes.NewReader(payload))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats["chunks"].(int64) == 0 {
		t.Fatal("expected chunks stored")
	}
}

func TestOpenFileSeek(t *testing.T) {
	s := openTestStorage(t)
	ctx := context.Background()
	data := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	man, err := s.Put(ctx, "seek.bin", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	rc, size, err := s.OpenFile(ctx, man.FileID)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if size != int64(len(data)) {
		t.Fatalf("size %d vs %d", size, len(data))
	}
	if _, err := rc.Seek(10, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data[10:]) {
		t.Fatalf("seek read mismatch: %q", got)
	}
}

func TestAtomicChunkWrite(t *testing.T) {
	dir := t.TempDir()
	s, err := storage.Open(storage.Config{RootDir: filepath.Join(dir, "storage")})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	_, err = s.Put(ctx, "a", bytes.NewReader(bytes.Repeat([]byte{1}, 70000)))
	if err != nil {
		t.Fatal(err)
	}
	// temp dir should not keep partials after success
	entries, _ := os.ReadDir(filepath.Join(dir, "storage", "temp"))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".partial" {
			t.Fatalf("leftover partial: %s", e.Name())
		}
	}
}
