package chunker_test

import (
	"bytes"
	"testing"

	"github.com/netodrive/server/internal/storage/chunker"
)

func TestFastCDCSplitsLargeInput(t *testing.T) {
	c := chunker.DefaultFastCDC()
	data := bytes.Repeat([]byte("x"), c.MaxSize*3)
	chunks, err := c.Chunk(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	total := 0
	for _, ch := range chunks {
		total += len(ch.Data)
	}
	if total != len(data) {
		t.Fatalf("chunk sizes sum %d != input %d", total, len(data))
	}
}

func TestFastCDCSmallFileSingleChunk(t *testing.T) {
	c := chunker.DefaultFastCDC()
	data := []byte("small file")
	chunks, err := c.Chunk(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !bytes.Equal(chunks[0].Data, data) {
		t.Fatal("chunk data mismatch")
	}
}
