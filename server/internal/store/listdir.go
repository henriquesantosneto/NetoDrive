package store

import (
	"sort"
	"strings"
)

// ListDir returns direct children of parent ("" = root).
// Intermediate folders are synthesized when files exist deeper in the tree
// (e.g. PC/doc.txt makes "PC" visible at root even without a directory row).
func (s *Store) ListDir(userID int64, parent string) ([]FileMeta, error) {
	parent = strings.Trim(parent, "/")
	all, err := s.ListAllActive(userID)
	if err != nil {
		return nil, err
	}

	byPath := make(map[string]FileMeta, len(all))
	for _, f := range all {
		byPath[f.Path] = f
	}

	type slot struct {
		meta  FileMeta
		isDir bool
	}
	children := map[string]slot{}

	for _, f := range all {
		rel := f.Path
		if parent != "" {
			if rel == parent {
				continue
			}
			prefix := parent + "/"
			if !strings.HasPrefix(rel, prefix) {
				continue
			}
			rel = strings.TrimPrefix(rel, prefix)
		}
		if rel == "" {
			continue
		}

		seg, rest, _ := strings.Cut(rel, "/")
		childPath := seg
		if parent != "" {
			childPath = parent + "/" + seg
		}

		if rest == "" {
			children[childPath] = slot{meta: f, isDir: f.IsDir}
			continue
		}

		if cur, ok := children[childPath]; ok && !cur.isDir && cur.meta.Path == childPath {
			continue
		}
		if explicit, ok := byPath[childPath]; ok {
			children[childPath] = slot{meta: explicit, isDir: explicit.IsDir}
			continue
		}
		children[childPath] = slot{
			meta: FileMeta{
				UserID: userID,
				Path:   childPath,
				Name:   seg,
				IsDir:  true,
				Mime:   "inode/directory",
			},
			isDir: true,
		}
	}

	out := make([]FileMeta, 0, len(children))
	for _, c := range children {
		out = append(out, c.meta)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}
