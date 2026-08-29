package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB      *sql.DB
	DataDir string
}

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type FileMeta struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	IsDir       bool      `json:"is_dir"`
	Size        int64     `json:"size"`
	Hash        string    `json:"hash"`
	Mime        string    `json:"mime"`
	MTime       time.Time `json:"mtime"`
	Deleted     bool      `json:"deleted"`
	Version     int64     `json:"version"`
	DeviceID    string    `json:"device_id,omitempty"`
	GalleryKey  string    `json:"gallery_key,omitempty"`
	Width       int       `json:"width,omitempty"`
	Height      int       `json:"height,omitempty"`
	TakenAt     *time.Time `json:"taken_at,omitempty"`
}

type Change struct {
	ID      int64    `json:"id"`
	UserID  int64    `json:"user_id"`
	FileID  int64    `json:"file_id"`
	Action  string   `json:"action"` // upsert | delete
	Version int64    `json:"version"`
	File    FileMeta `json:"file"`
}

func Open(dbPath, dataDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "blobs"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "thumbs"), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{DB: db, DataDir: dataDir}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id),
  path TEXT NOT NULL,
  name TEXT NOT NULL,
  is_dir INTEGER NOT NULL DEFAULT 0,
  size INTEGER NOT NULL DEFAULT 0,
  hash TEXT NOT NULL DEFAULT '',
  mime TEXT NOT NULL DEFAULT 'application/octet-stream',
  mtime TEXT NOT NULL,
  deleted INTEGER NOT NULL DEFAULT 0,
  version INTEGER NOT NULL DEFAULT 1,
  device_id TEXT NOT NULL DEFAULT '',
  gallery_key TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  taken_at TEXT,
  UNIQUE(user_id, path)
);

CREATE TABLE IF NOT EXISTS changes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id),
  file_id INTEGER NOT NULL,
  action TEXT NOT NULL,
  version INTEGER NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_files_user_path ON files(user_id, path);
CREATE INDEX IF NOT EXISTS idx_files_gallery ON files(user_id, gallery_key);
CREATE INDEX IF NOT EXISTS idx_changes_user_id ON changes(user_id, id);
`
	_, err := s.DB.Exec(schema)
	return err
}

func (s *Store) BlobPath(hash string) string {
	if len(hash) < 4 {
		return filepath.Join(s.DataDir, "blobs", hash)
	}
	return filepath.Join(s.DataDir, "blobs", hash[:2], hash[2:4], hash)
}

func (s *Store) EnsureBlobDir(hash string) error {
	return os.MkdirAll(filepath.Dir(s.BlobPath(hash)), 0o755)
}

func (s *Store) CreateUser(username, passwordHash string) (*User, error) {
	now := time.Now().UTC()
	res, err := s.DB.Exec(
		`INSERT INTO users(username, password_hash, created_at) VALUES(?,?,?)`,
		username, passwordHash, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, PasswordHash: passwordHash, CreatedAt: now}, nil
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE username=?`, username)
	var u User
	var created string
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &created); err != nil {
		return nil, err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &u, nil
}

func (s *Store) GetUserByID(id int64) (*User, error) {
	row := s.DB.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE id=?`, id)
	var u User
	var created string
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &created); err != nil {
		return nil, err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &u, nil
}

func (s *Store) NextVersion(userID int64) (int64, error) {
	var v sql.NullInt64
	err := s.DB.QueryRow(`SELECT MAX(version) FROM files WHERE user_id=?`, userID).Scan(&v)
	if err != nil {
		return 0, err
	}
	if !v.Valid {
		return 1, nil
	}
	return v.Int64 + 1, nil
}

func scanFile(scanner interface {
	Scan(dest ...any) error
}) (FileMeta, error) {
	var f FileMeta
	var isDir, deleted int
	var mtime, taken sql.NullString
	err := scanner.Scan(
		&f.ID, &f.UserID, &f.Path, &f.Name, &isDir, &f.Size, &f.Hash, &f.Mime,
		&mtime, &deleted, &f.Version, &f.DeviceID, &f.GalleryKey, &f.Width, &f.Height, &taken,
	)
	if err != nil {
		return f, err
	}
	f.IsDir = isDir == 1
	f.Deleted = deleted == 1
	if mtime.Valid {
		f.MTime, _ = time.Parse(time.RFC3339Nano, mtime.String)
	}
	if taken.Valid && taken.String != "" {
		t, err := time.Parse(time.RFC3339Nano, taken.String)
		if err == nil {
			f.TakenAt = &t
		}
	}
	return f, nil
}

func (s *Store) GetFileByPath(userID int64, path string) (*FileMeta, error) {
	row := s.DB.QueryRow(`
SELECT id, user_id, path, name, is_dir, size, hash, mime, mtime, deleted, version,
       device_id, gallery_key, width, height, taken_at
FROM files WHERE user_id=? AND path=?`, userID, path)
	f, err := scanFile(row)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Store) GetFileByID(userID, id int64) (*FileMeta, error) {
	row := s.DB.QueryRow(`
SELECT id, user_id, path, name, is_dir, size, hash, mime, mtime, deleted, version,
       device_id, gallery_key, width, height, taken_at
FROM files WHERE user_id=? AND id=?`, userID, id)
	f, err := scanFile(row)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Store) ListChildren(userID int64, parent string) ([]FileMeta, error) {
	rows, err := s.DB.Query(`
SELECT id, user_id, path, name, is_dir, size, hash, mime, mtime, deleted, version,
       device_id, gallery_key, width, height, taken_at
FROM files
WHERE user_id=? AND deleted=0 AND path LIKE ? AND path NOT LIKE ?
ORDER BY is_dir DESC, name COLLATE NOCASE ASC`,
		userID, parent+"/%", parent+"/%/%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FileMeta
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		// For root listing, parent is ""
		if parent == "" {
			if filepath.Dir(f.Path) != "." && filepath.Dir(f.Path) != "/" {
				// only direct children of root: no slash beyond first segment
				rest := stringsTrimPrefix(f.Path, "/")
				if containsSlash(rest) {
					continue
				}
			}
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func stringsTrimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func containsSlash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return true
		}
	}
	return false
}

func (s *Store) ListAllActive(userID int64) ([]FileMeta, error) {
	rows, err := s.DB.Query(`
SELECT id, user_id, path, name, is_dir, size, hash, mime, mtime, deleted, version,
       device_id, gallery_key, width, height, taken_at
FROM files WHERE user_id=? AND deleted=0 ORDER BY path`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FileMeta
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) UpsertFile(f *FileMeta) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	ver, err := nextVersionTx(tx, f.UserID)
	if err != nil {
		return err
	}
	f.Version = ver
	f.Deleted = false

	var taken any
	if f.TakenAt != nil {
		taken = f.TakenAt.UTC().Format(time.RFC3339Nano)
	}

	res, err := tx.Exec(`
INSERT INTO files(user_id, path, name, is_dir, size, hash, mime, mtime, deleted, version,
                  device_id, gallery_key, width, height, taken_at)
VALUES(?,?,?,?,?,?,?,?,0,?,?,?,?,?,?)
ON CONFLICT(user_id, path) DO UPDATE SET
  name=excluded.name,
  is_dir=excluded.is_dir,
  size=excluded.size,
  hash=excluded.hash,
  mime=excluded.mime,
  mtime=excluded.mtime,
  deleted=0,
  version=excluded.version,
  device_id=excluded.device_id,
  gallery_key=excluded.gallery_key,
  width=excluded.width,
  height=excluded.height,
  taken_at=excluded.taken_at
`,
		f.UserID, f.Path, f.Name, boolToInt(f.IsDir), f.Size, f.Hash, f.Mime,
		f.MTime.UTC().Format(time.RFC3339Nano), f.Version, f.DeviceID, f.GalleryKey,
		f.Width, f.Height, taken,
	)
	if err != nil {
		return err
	}
	_ = res

	row := tx.QueryRow(`SELECT id FROM files WHERE user_id=? AND path=?`, f.UserID, f.Path)
	if err := row.Scan(&f.ID); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`INSERT INTO changes(user_id, file_id, action, version, created_at) VALUES(?,?,?,?,?)`,
		f.UserID, f.ID, "upsert", f.Version, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SoftDelete(userID int64, path string) (*FileMeta, error) {
	f, err := s.GetFileByPath(userID, path)
	if err != nil {
		return nil, err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	ver, err := nextVersionTx(tx, userID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE files SET deleted=1, version=? WHERE id=?`, ver, f.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`INSERT INTO changes(user_id, file_id, action, version, created_at) VALUES(?,?,?,?,?)`,
		userID, f.ID, "delete", ver, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	f.Deleted = true
	f.Version = ver
	return f, nil
}

func nextVersionTx(tx *sql.Tx, userID int64) (int64, error) {
	var v sql.NullInt64
	err := tx.QueryRow(`SELECT MAX(version) FROM files WHERE user_id=?`, userID).Scan(&v)
	if err != nil {
		return 0, err
	}
	if !v.Valid {
		return 1, nil
	}
	return v.Int64 + 1, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) ChangesSince(userID, sinceID int64, limit int) ([]Change, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := s.DB.Query(`
SELECT c.id, c.user_id, c.file_id, c.action, c.version,
       f.id, f.user_id, f.path, f.name, f.is_dir, f.size, f.hash, f.mime, f.mtime, f.deleted, f.version,
       f.device_id, f.gallery_key, f.width, f.height, f.taken_at
FROM changes c
JOIN files f ON f.id = c.file_id
WHERE c.user_id=? AND c.id > ?
ORDER BY c.id ASC
LIMIT ?`, userID, sinceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Change
	for rows.Next() {
		var c Change
		var isDir, deleted int
		var mtime, taken sql.NullString
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.FileID, &c.Action, &c.Version,
			&c.File.ID, &c.File.UserID, &c.File.Path, &c.File.Name, &isDir, &c.File.Size, &c.File.Hash, &c.File.Mime,
			&mtime, &deleted, &c.File.Version, &c.File.DeviceID, &c.File.GalleryKey, &c.File.Width, &c.File.Height, &taken,
		); err != nil {
			return nil, err
		}
		c.File.IsDir = isDir == 1
		c.File.Deleted = deleted == 1
		if mtime.Valid {
			c.File.MTime, _ = time.Parse(time.RFC3339Nano, mtime.String)
		}
		if taken.Valid && taken.String != "" {
			t, err := time.Parse(time.RFC3339Nano, taken.String)
			if err == nil {
				c.File.TakenAt = &t
			}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) ListGallery(userID int64, limit, offset int) ([]FileMeta, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.DB.Query(`
SELECT id, user_id, path, name, is_dir, size, hash, mime, mtime, deleted, version,
       device_id, gallery_key, width, height, taken_at
FROM files
WHERE user_id=? AND deleted=0 AND gallery_key != '' AND mime LIKE 'image/%'
ORDER BY COALESCE(taken_at, mtime) DESC
LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FileMeta
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) EnsureParentDirs(userID int64, filePath, deviceID string) error {
	dir := filepath.ToSlash(filepath.Dir(filePath))
	if dir == "." || dir == "/" || dir == "" {
		return nil
	}
	parts := splitPath(dir)
	cur := ""
	for _, p := range parts {
		if cur == "" {
			cur = p
		} else {
			cur = cur + "/" + p
		}
		existing, err := s.GetFileByPath(userID, cur)
		if err == nil && existing != nil && !existing.Deleted {
			continue
		}
		meta := &FileMeta{
			UserID:   userID,
			Path:     cur,
			Name:     p,
			IsDir:    true,
			Mime:     "inode/directory",
			MTime:    time.Now().UTC(),
			DeviceID: deviceID,
		}
		if err := s.UpsertFile(meta); err != nil {
			return fmt.Errorf("ensure dir %s: %w", cur, err)
		}
	}
	return nil
}

func splitPath(p string) []string {
	p = filepath.ToSlash(p)
	for len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	if p == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			if i > start {
				parts = append(parts, p[start:i])
			}
			start = i + 1
		}
	}
	if start < len(p) {
		parts = append(parts, p[start:])
	}
	return parts
}
