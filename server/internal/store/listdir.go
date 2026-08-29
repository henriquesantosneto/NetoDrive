package store

import (
	"path"
	"strings"
)

// ListDir returns direct children of parent ("" = root).
func (s *Store) ListDir(userID int64, parent string) ([]FileMeta, error) {
	parent = strings.Trim(parent, "/")
	all, err := s.ListAllActive(userID)
	if err != nil {
		return nil, err
	}
	var out []FileMeta
	for _, f := range all {
		dir := path.Dir(f.Path)
		if dir == "." {
			dir = ""
		}
		if dir == parent {
			out = append(out, f)
		}
	}
	return out, nil
}
