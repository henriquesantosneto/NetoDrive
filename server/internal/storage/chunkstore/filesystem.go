package chunkstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Filesystem stores compressed chunks on a local filesystem.
type Filesystem struct {
	root string
}

func NewFilesystem(root string) (*Filesystem, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(filepath.Join(root, "chunks"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "temp"), 0o755); err != nil {
		return nil, err
	}
	return &Filesystem{root: root}, nil
}

func (fs *Filesystem) Root() string { return fs.root }

// PathForHash derives the final chunk path from its content hash.
func PathForHash(root, hash string) string {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if len(hash) < 4 {
		return filepath.Join(root, "chunks", hash+".chunk")
	}
	return filepath.Join(root, "chunks", hash[:2], hash[2:4], hash+".chunk")
}

func (fs *Filesystem) FinalPath(hash string) string {
	return PathForHash(fs.root, hash)
}

func (fs *Filesystem) TempPath(hash string) string {
	hash = strings.ToLower(strings.TrimSpace(hash))
	return filepath.Join(fs.root, "temp", hash+".partial")
}

// PutAtomic writes data via temp + rename into the hash-derived path.
func (fs *Filesystem) PutAtomic(hash string, data []byte) (string, error) {
	final := fs.FinalPath(hash)
	if _, err := os.Stat(final); err == nil {
		return final, nil
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return "", err
	}
	tmp := fs.TempPath(hash)
	if err := os.MkdirAll(filepath.Dir(tmp), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, final); err != nil {
		// Another writer may have won the race.
		if _, statErr := os.Stat(final); statErr == nil {
			_ = os.Remove(tmp)
			return final, nil
		}
		return "", fmt.Errorf("atomic rename chunk %s: %w", hash, err)
	}
	return final, nil
}

func (fs *Filesystem) Read(hash string) ([]byte, error) {
	path := fs.FinalPath(hash)
	return os.ReadFile(path)
}

func (fs *Filesystem) Remove(hash string) error {
	path := fs.FinalPath(hash)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (fs *Filesystem) Exists(hash string) bool {
	_, err := os.Stat(fs.FinalPath(hash))
	return err == nil
}
