package cachelru

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Disk mirrors the Android MediaCache LRU behaviour for verification.
type Disk struct {
	Root   string
	Budget int64
}

func (d *Disk) Put(key string, data []byte) error {
	path := filepath.Join(d.Root, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	_ = os.Chtimes(path, time.Now(), time.Now())
	return d.Trim()
}

func (d *Disk) Touch(key string) {
	path := filepath.Join(d.Root, key)
	_ = os.Chtimes(path, time.Now(), time.Now())
}

func (d *Disk) Has(key string) bool {
	_, err := os.Stat(filepath.Join(d.Root, key))
	return err == nil
}

func (d *Disk) Usage() int64 {
	var total int64
	_ = filepath.Walk(d.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func (d *Disk) Trim() error {
	type entry struct {
		path string
		mod  time.Time
		size int64
	}
	var files []entry
	_ = filepath.Walk(d.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		files = append(files, entry{path: path, mod: info.ModTime(), size: info.Size()})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	total := d.Usage()
	for _, f := range files {
		if total <= d.Budget {
			break
		}
		if err := os.Remove(f.path); err == nil {
			total -= f.size
		}
	}
	return nil
}
