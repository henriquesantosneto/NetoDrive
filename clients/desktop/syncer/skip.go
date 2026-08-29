package syncer

import (
	"path/filepath"
	"strings"
)

// skipDirNames are never descended into during sync.
var skipDirNames = map[string]bool{
	".git":         true,
	".cursor":      true,
	"node_modules": true,
	"__pycache__":  true,
	".idea":        true,
	"dist":         true,
	"target":       true,
}

func shouldSkipWalkEntry(localRoot, absPath string, name string, isDir bool) bool {
	if name == "" {
		return false
	}
	// Hidden files/dirs at any level
	if strings.HasPrefix(name, ".") {
		return true
	}
	if isDir && skipDirNames[name] {
		return true
	}
	rel, err := filepath.Rel(localRoot, absPath)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
		if skipDirNames[part] {
			return true
		}
	}
	return false
}
