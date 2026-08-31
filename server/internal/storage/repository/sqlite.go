package repository

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite persists chunk and file manifests.
type SQLite struct {
	db *sql.DB
}

func OpenSQLite(dbPath string) (*SQLite, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	r := &SQLite{db: db}
	if err := r.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return r, nil
}

func (r *SQLite) DB() *sql.DB { return r.db }

func (r *SQLite) Close() error { return r.db.Close() }

func (r *SQLite) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS chunks (
    hash TEXT PRIMARY KEY,
    size INTEGER NOT NULL,
    compressed_size INTEGER NOT NULL,
    compression TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    ref_count INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    size INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS file_chunks (
    file_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    chunk_hash TEXT NOT NULL,
    PRIMARY KEY (file_id, chunk_index),
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE,
    FOREIGN KEY (chunk_hash) REFERENCES chunks(hash)
);

CREATE INDEX IF NOT EXISTS idx_file_chunks_file ON file_chunks(file_id);
CREATE INDEX IF NOT EXISTS idx_file_chunks_hash ON file_chunks(chunk_hash);
CREATE INDEX IF NOT EXISTS idx_chunks_ref ON chunks(ref_count);
`
	_, err := r.db.Exec(schema)
	return err
}

func (r *SQLite) GetChunk(hash string) (*ChunkRecord, error) {
	row := r.db.QueryRow(`
SELECT hash, size, compressed_size, compression, storage_path, ref_count, created_at
FROM chunks WHERE hash=?`, hash)
	var rec ChunkRecord
	if err := row.Scan(&rec.Hash, &rec.Size, &rec.CompressedSize, &rec.Compression,
		&rec.StoragePath, &rec.RefCount, &rec.CreatedAt); err != nil {
		return nil, err
	}
	return &rec, nil
}

// UpsertChunk inserts or returns existing chunk metadata.
func (r *SQLite) UpsertChunk(rec ChunkRecord) error {
	_, err := r.db.Exec(`
INSERT INTO chunks(hash, size, compressed_size, compression, storage_path, ref_count, created_at)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(hash) DO NOTHING`,
		rec.Hash, rec.Size, rec.CompressedSize, rec.Compression, rec.StoragePath, rec.RefCount, rec.CreatedAt,
	)
	return err
}

// IncRefCount atomically increments ref_count for a chunk.
func (r *SQLite) IncRefCount(hash string, delta int64) error {
	res, err := r.db.Exec(`UPDATE chunks SET ref_count = ref_count + ? WHERE hash=?`, delta, hash)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("chunk not found: %s", hash)
	}
	return nil
}

func (r *SQLite) CreateFile(id, name string, size int64, chunks []FileChunkRef) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	if _, err := tx.Exec(`
INSERT INTO files(id, name, size, created_at, updated_at) VALUES(?,?,?,?,?)`,
		id, name, size, now, now); err != nil {
		return err
	}
	for _, ch := range chunks {
		if _, err := tx.Exec(`
INSERT INTO file_chunks(file_id, chunk_index, chunk_hash) VALUES(?,?,?)`,
			id, ch.Index, ch.Hash); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLite) GetManifest(fileID string) (*FileManifest, error) {
	row := r.db.QueryRow(`SELECT id, size FROM files WHERE id=?`, fileID)
	var man FileManifest
	if err := row.Scan(&man.FileID, &man.Size); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
SELECT fc.chunk_index, fc.chunk_hash, c.size
FROM file_chunks fc
JOIN chunks c ON c.hash = fc.chunk_hash
WHERE fc.file_id=?
ORDER BY fc.chunk_index ASC`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref FileChunkRef
		if err := rows.Scan(&ref.Index, &ref.Hash, &ref.Size); err != nil {
			return nil, err
		}
		man.Chunks = append(man.Chunks, ref)
	}
	return &man, rows.Err()
}

func (r *SQLite) DeleteFile(fileID string) ([]string, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.Query(`SELECT chunk_hash FROM file_chunks WHERE file_id=?`, fileID)
	if err != nil {
		return nil, err
	}
	var hashes []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			_ = rows.Close()
			return nil, err
		}
		hashes = append(hashes, h)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`DELETE FROM files WHERE id=?`, fileID); err != nil {
		return nil, err
	}
	for _, h := range hashes {
		if _, err := tx.Exec(`UPDATE chunks SET ref_count = ref_count - 1 WHERE hash=?`, h); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return hashes, nil
}

func (r *SQLite) ListOrphanChunks(limit int) ([]ChunkRecord, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := r.db.Query(`
SELECT hash, size, compressed_size, compression, storage_path, ref_count, created_at
FROM chunks WHERE ref_count <= 0 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChunkRecord
	for rows.Next() {
		var rec ChunkRecord
		if err := rows.Scan(&rec.Hash, &rec.Size, &rec.CompressedSize, &rec.Compression,
			&rec.StoragePath, &rec.RefCount, &rec.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *SQLite) DeleteChunkRecord(hash string) error {
	_, err := r.db.Exec(`DELETE FROM chunks WHERE hash=? AND ref_count <= 0`, hash)
	return err
}

func (r *SQLite) ForceDeleteChunkRecord(hash string) error {
	_, err := r.db.Exec(`DELETE FROM chunks WHERE hash=?`, hash)
	return err
}

func (r *SQLite) ChunkStillOrphan(hash string) (bool, error) {
	var rc int64
	err := r.db.QueryRow(`SELECT ref_count FROM chunks WHERE hash=?`, hash).Scan(&rc)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return rc <= 0, nil
}

// ReserveNewChunk inserts chunk with ref_count=0 before physical write (dedup gate).
func (r *SQLite) ReserveNewChunk(rec ChunkRecord) (inserted bool, err error) {
	res, err := r.db.Exec(`
INSERT INTO chunks(hash, size, compressed_size, compression, storage_path, ref_count, created_at)
VALUES(?,?,?,?,?,0,?)
ON CONFLICT(hash) DO NOTHING`,
		rec.Hash, rec.Size, rec.CompressedSize, rec.Compression, rec.StoragePath, rec.CreatedAt,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *SQLite) WithTx(fn func(tx *sql.Tx) error) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
