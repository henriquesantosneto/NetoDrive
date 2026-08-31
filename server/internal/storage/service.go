package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/netodrive/server/internal/storage/chunker"
	"github.com/netodrive/server/internal/storage/chunkstore"
	"github.com/netodrive/server/internal/storage/compressor"
	"github.com/netodrive/server/internal/storage/hasher"
	"github.com/netodrive/server/internal/storage/repository"
)

// Service implements content-defined chunk storage with dedup and compression.
type Service struct {
	repo   *repository.SQLite
	store  *chunkstore.Filesystem
	chunk  *chunker.FastCDC
	hash   Hasher
	comp   Compressor
	mu     sync.Mutex
	inGC   bool
}

// Config for opening chunk storage.
type Config struct {
	RootDir string
	DBPath  string
}

// Open creates the storage layer under root (chunks/, temp/, storage.db).
func Open(cfg Config) (*Service, error) {
	if cfg.RootDir == "" {
		return nil, fmt.Errorf("storage root required")
	}
	if cfg.DBPath == "" {
		cfg.DBPath = cfg.RootDir + "/storage.db"
	}
	repo, err := repository.OpenSQLite(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	fs, err := chunkstore.NewFilesystem(cfg.RootDir)
	if err != nil {
		_ = repo.Close()
		return nil, err
	}
	comp, err := compressor.NewZstd()
	if err != nil {
		_ = repo.Close()
		return nil, err
	}
	return &Service{
		repo:  repo,
		store: fs,
		chunk: chunker.DefaultFastCDC(),
		hash:  hasher.NewSHA256(),
		comp:  comp,
	}, nil
}

func (s *Service) Close() error {
	if z, ok := s.comp.(interface{ Close() error }); ok {
		_ = z.Close()
	}
	return s.repo.Close()
}

// Put stores a file by chunking, deduplicating, and compressing.
func (s *Service) Put(ctx context.Context, name string, r io.Reader) (FileManifest, error) {
	if err := ctx.Err(); err != nil {
		return FileManifest{}, err
	}
	chunks, err := s.chunk.Chunk(r)
	if err != nil {
		return FileManifest{}, err
	}
	fileID := uuid.NewString()
	var refs []ChunkRef
	var total int64

	for _, ch := range chunks {
		if err := ctx.Err(); err != nil {
			return FileManifest{}, err
		}
		hash := s.hash.Sum(ch.Data)
		size := int64(len(ch.Data))
		if err := s.ensureChunk(ctx, hash, ch.Data); err != nil {
			return FileManifest{}, err
		}
		if err := s.repo.IncRefCount(hash, 1); err != nil {
			return FileManifest{}, err
		}
		refs = append(refs, ChunkRef{Index: ch.Index, Hash: hash, Size: size})
		total += size
	}

	if err := s.repo.CreateFile(fileID, name, total, toRepoRefs(refs)); err != nil {
		for _, ref := range refs {
			_ = s.repo.IncRefCount(ref.Hash, -1)
		}
		return FileManifest{}, err
	}
	return FileManifest{FileID: fileID, Size: total, Chunks: refs}, nil
}

func (s *Service) ensureChunk(ctx context.Context, hash string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.store.Exists(hash) {
		return nil
	}
	if rec, err := s.repo.GetChunk(hash); err == nil {
		if s.store.Exists(hash) {
			return nil
		}
		// Stale metadata from a crashed writer before the physical file landed.
		if rec.RefCount <= 0 {
			_ = s.repo.ForceDeleteChunkRecord(hash)
		}
	}

	compressed, err := s.comp.Compress(data)
	if err != nil {
		return err
	}
	path := s.store.FinalPath(hash)
	rec := repository.ChunkRecord{
		Hash:           hash,
		Size:           int64(len(data)),
		CompressedSize: int64(len(compressed)),
		Compression:    s.comp.Algorithm(),
		StoragePath:    path,
		CreatedAt:      time.Now().Unix(),
	}

	inserted, err := s.repo.ReserveNewChunk(rec)
	if err != nil {
		return err
	}
	if !inserted {
		if s.store.Exists(hash) {
			return nil
		}
		return s.waitForChunk(ctx, hash)
	}

	if _, err := s.store.PutAtomic(hash, compressed); err != nil {
		_ = s.repo.ForceDeleteChunkRecord(hash)
		return err
	}
	return nil
}

func (s *Service) waitForChunk(ctx context.Context, hash string) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.store.Exists(hash) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for chunk %s", hash)
}

// GetChunk returns decompressed chunk bytes after integrity verification.
func (s *Service) GetChunk(ctx context.Context, hash string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rec, err := s.repo.GetChunk(hash)
	if err != nil {
		return nil, err
	}
	compressed, err := s.store.Read(hash)
	if err != nil {
		return nil, fmt.Errorf("read chunk %s: %w", hash, err)
	}
	raw, err := s.comp.Decompress(compressed)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) != rec.Size {
		return nil, fmt.Errorf("chunk %s size mismatch: db=%d got=%d", hash, rec.Size, len(raw))
	}
	if err := s.hash.Verify(raw, hash); err != nil {
		return nil, fmt.Errorf("integrity: %w", err)
	}
	_ = rec.StoragePath
	return io.NopCloser(bytes.NewReader(raw)), nil
}

// GetManifest returns the chunk sequence for a stored file.
func (s *Service) GetManifest(ctx context.Context, fileID string) (FileManifest, error) {
	if err := ctx.Err(); err != nil {
		return FileManifest{}, err
	}
	man, err := s.repo.GetManifest(fileID)
	if err != nil {
		return FileManifest{}, err
	}
	return FileManifest{FileID: man.FileID, Size: man.Size, Chunks: fromRepoRefs(man.Chunks)}, nil
}

func toRepoRefs(refs []ChunkRef) []repository.FileChunkRef {
	out := make([]repository.FileChunkRef, len(refs))
	for i, r := range refs {
		out[i] = repository.FileChunkRef{Index: r.Index, Hash: r.Hash, Size: r.Size}
	}
	return out
}

func fromRepoRefs(refs []repository.FileChunkRef) []ChunkRef {
	out := make([]ChunkRef, len(refs))
	for i, r := range refs {
		out[i] = ChunkRef{Index: r.Index, Hash: r.Hash, Size: r.Size}
	}
	return out
}

// ReadFile reassembles the full file from its manifest.
func (s *Service) ReadFile(ctx context.Context, fileID string) (io.ReadCloser, int64, error) {
	man, err := s.GetManifest(ctx, fileID)
	if err != nil {
		return nil, 0, err
	}
	var buf bytes.Buffer
	for _, ref := range man.Chunks {
		rc, err := s.GetChunk(ctx, ref.Hash)
		if err != nil {
			return nil, 0, err
		}
		if _, err := io.Copy(&buf, rc); err != nil {
			_ = rc.Close()
			return nil, 0, err
		}
		_ = rc.Close()
	}
	if int64(buf.Len()) != man.Size {
		return nil, 0, fmt.Errorf("file %s size mismatch: expected %d got %d", fileID, man.Size, buf.Len())
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes())), man.Size, nil
}

// DeleteFile removes a file manifest and decrements chunk ref counts.
func (s *Service) DeleteFile(ctx context.Context, fileID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.repo.DeleteFile(fileID)
	return err
}

// RunGC removes chunks with ref_count=0 using a two-phase safe process.
func (s *Service) RunGC(ctx context.Context) error {
	s.mu.Lock()
	if s.inGC {
		s.mu.Unlock()
		return fmt.Errorf("gc already running")
	}
	s.inGC = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.inGC = false
		s.mu.Unlock()
	}()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		orphans, err := s.repo.ListOrphanChunks(100)
		if err != nil {
			return err
		}
		if len(orphans) == 0 {
			return nil
		}
		for _, ch := range orphans {
			if err := ctx.Err(); err != nil {
				return err
			}
			still, err := s.repo.ChunkStillOrphan(ch.Hash)
			if err != nil {
				return err
			}
			if !still {
				continue
			}
			if err := s.store.Remove(ch.Hash); err != nil {
				return err
			}
			if err := s.repo.DeleteChunkRecord(ch.Hash); err != nil {
				return err
			}
		}
	}
}

// Stats returns basic storage counters for diagnostics.
func (s *Service) Stats(ctx context.Context) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var files, chunks, orphans int64
	_ = s.repo.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM files`).Scan(&files)
	_ = s.repo.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks`).Scan(&chunks)
	_ = s.repo.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks WHERE ref_count <= 0`).Scan(&orphans)
	return map[string]any{
		"files":   files,
		"chunks":  chunks,
		"orphans": orphans,
	}, nil
}
