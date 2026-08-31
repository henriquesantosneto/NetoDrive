package storage

import (
	"context"
	"io"

	"github.com/netodrive/server/internal/storage/chunker"
)

// Storage is the content-addressed chunk storage API.
type Storage interface {
	Put(ctx context.Context, name string, r io.Reader) (FileManifest, error)
	GetChunk(ctx context.Context, hash string) (io.ReadCloser, error)
	DeleteFile(ctx context.Context, fileID string) error
	GetManifest(ctx context.Context, fileID string) (FileManifest, error)
	RunGC(ctx context.Context) error
}

// FileManifest describes a content-addressed file as an ordered chunk sequence.
type FileManifest struct {
	FileID string
	Size   int64
	Chunks []ChunkRef
}

// ChunkRef references one chunk in a file manifest.
type ChunkRef struct {
	Index int
	Hash  string
	Size  int64
}

// Chunker splits readers into variable-size chunks.
type Chunker interface {
	Chunk(r io.Reader) ([]chunker.ChunkData, error)
}

// Hasher computes content identity hashes.
type Hasher interface {
	Sum(data []byte) string
	Verify(data []byte, expected string) error
}

// Compressor compresses chunk payloads for storage.
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Algorithm() string
}
