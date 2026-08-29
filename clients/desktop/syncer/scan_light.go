package syncer

import (
	"os"
	"path/filepath"
	"strings"
)

// scanLocalFilesLight builds the local index without a full tree walk (avoids CFAPI/Explorer hangs).
func scanLocalFilesLight(localRoot string, known map[string]string) (map[string]string, error) {
	local := indexMetaStore(localRoot)

	// CFAPI sync root: meta + sync-state only — zero access to the sync folder (ReadDir/Stat freeze Explorer).
	if cfapiProviderActive() {
		known = filterKnownExcludingDeletes(localRoot, known)
		for rel, hash := range known {
			if _, ok := local[rel]; ok {
				continue
			}
			if hash != "" {
				local[rel] = hash
			}
		}
		return local, nil
	}

	for rel, oldHash := range known {
		if _, ok := local[rel]; ok {
			continue
		}
		path := placeholderPath(localRoot, rel)
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		if st.IsDir() {
			continue
		}
		if isPlatformPlaceholder(path) {
			if meta, ok := readPlaceholderMetaForRel(localRoot, rel); ok {
				local[rel] = meta.Hash
			}
			continue
		}
		h, _, err := FileHash(path)
		if err != nil {
			if oldHash != "" {
				local[rel] = oldHash
			}
			continue
		}
		local[rel] = h
	}

	shallow, err := scanShallowNewFiles(localRoot, local)
	for k, v := range shallow {
		local[k] = v
	}
	return local, err
}

func scanShallowNewFiles(localRoot string, existing map[string]string) (map[string]string, error) {
	found := map[string]string{}
	err := filepath.WalkDir(localRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if shouldSkipWalkEntry(localRoot, path, name, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(localRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "..") {
			return nil
		}
		depth := strings.Count(rel, "/") + 1
		if info.IsDir() {
			if depth >= 2 {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := existing[rel]; ok {
			return nil
		}
		if isPlatformPlaceholder(path) {
			if meta, ok := readPlaceholderMetaForPath(localRoot, path, rel); ok {
				found[rel] = meta.Hash
			}
			return nil
		}
		h, _, err := FileHash(path)
		if err != nil {
			return nil
		}
		found[rel] = h
		return nil
	})
	return found, err
}
