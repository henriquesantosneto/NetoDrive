package repository

// ChunkRecord is chunk metadata persisted in SQLite.
type ChunkRecord struct {
	Hash           string
	Size           int64
	CompressedSize int64
	Compression    string
	StoragePath    string
	RefCount       int64
	CreatedAt      int64
}

// FileChunkRef links a file to a chunk in manifest order.
type FileChunkRef struct {
	Index int
	Hash  string
	Size  int64
}
