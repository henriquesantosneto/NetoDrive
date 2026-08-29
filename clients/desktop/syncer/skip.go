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
	".netodrive":   true,
}

// repoTopSkipDirs: when local_folder is the git clone, do not sync source trees as user files.
var repoTopSkipDirs = map[string]bool{
	"clients":  true,
	"server":   true,
	"web":      true,
	"scripts":  true,
	"docs":     true,
	"vendor":   true,
	"android":  true,
	"ios":      true,
	"internal": true,
}

var syncWalkLocalRoot string
var syncWalkLocalRootIsRepo bool
var repoRootCache = map[string]bool{}

func setSyncWalkContext(localRoot string, st *SyncState) {
	syncWalkLocalRoot = localRoot
	if st != nil && st.IsRepoRoot != nil {
		syncWalkLocalRootIsRepo = *st.IsRepoRoot
		repoRootCache[localRoot] = *st.IsRepoRoot
		return
	}
	if v, ok := repoRootCache[localRoot]; ok {
		syncWalkLocalRootIsRepo = v
		if st != nil && st.IsRepoRoot == nil {
			b := v
			st.IsRepoRoot = &b
		}
		return
	}
	// Stat the sync root once per process; repeat Stat under CFAPI freezes Explorer.
	v := IsLikelyRepoRoot(localRoot)
	repoRootCache[localRoot] = v
	syncWalkLocalRootIsRepo = v
	if st != nil {
		b := v
		st.IsRepoRoot = &b
	}
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
	if syncWalkLocalRootIsRepo && isDir && localRoot == syncWalkLocalRoot {
		if repoTopSkipDirs[name] {
			return true
		}
	}
	rel, err := filepath.Rel(localRoot, absPath)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return false
	}
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
