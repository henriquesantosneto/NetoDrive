package syncer

import (
	"os"
	"path/filepath"
	"strings"
)

const dirMarker = "__netodrive_dir__"

type dirSyncPlan struct {
	upload       []string
	download     []string
	deleteLocal  []string
	deleteRemote []string
}

func planDirSync(local, remote, known map[string]bool) dirSyncPlan {
	var p dirSyncPlan
	for rel := range local {
		if remote[rel] {
			continue
		}
		if known[rel] {
			p.deleteLocal = append(p.deleteLocal, rel)
			continue
		}
		p.upload = append(p.upload, rel)
	}
	for rel := range remote {
		if local[rel] {
			continue
		}
		if known[rel] {
			p.deleteRemote = append(p.deleteRemote, rel)
			continue
		}
		p.download = append(p.download, rel)
	}
	return p
}

func dirsChanged(local, known map[string]bool) bool {
	if len(local) != len(known) {
		return true
	}
	for rel := range local {
		if !known[rel] {
			return true
		}
	}
	for rel := range known {
		if !local[rel] {
			return true
		}
	}
	return false
}

func scanLocalDirsLight(localRoot string) (map[string]bool, error) {
	found := map[string]bool{}
	err := filepath.WalkDir(localRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		rel, err := filepath.Rel(localRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
			return nil
		}
		if shouldSkipWalkEntry(localRoot, path, name, true) {
			return filepath.SkipDir
		}
		found[rel] = true
		return nil
	})
	return found, err
}

func ensureKnownDirs(st *SyncState) {
	if st.KnownDirs == nil {
		st.KnownDirs = map[string]bool{}
	}
}

func deleteLocalDir(localRoot, rel string) error {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	if rel == "" {
		return nil
	}
	abs := filepath.Join(localRoot, filepath.FromSlash(rel))
	if cfapiProviderActive() {
		entries, err := os.ReadDir(abs)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if len(entries) > 0 {
			return nil
		}
	}
	return os.Remove(abs)
}

func createLocalDir(localRoot, rel string) error {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	if rel == "" {
		return nil
	}
	return os.MkdirAll(filepath.Join(localRoot, filepath.FromSlash(rel)), 0o755)
}

func sortDirsByDepth(paths []string) {
	// Parent paths before children for remote mkdir.
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			di := strings.Count(paths[i], "/")
			dj := strings.Count(paths[j], "/")
			if dj < di {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}
}
