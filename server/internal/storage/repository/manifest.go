package repository

// FileManifest describes a stored file as ordered chunks.
type FileManifest struct {
	FileID string
	Size   int64
	Chunks []FileChunkRef
}
