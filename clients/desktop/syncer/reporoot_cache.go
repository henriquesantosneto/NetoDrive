package syncer

import (
	"path/filepath"
	"strings"
)

// PrepareRepoRootCache seeds repo-root detection without Stat under CFAPI sync roots.
// Pass cfgIsRepo from netodrive.json "is_repo_root" when local_folder is the git clone.
func PrepareRepoRootCache(localRoot, statePath string, cfgIsRepo *bool) {
	key := normalizeRootKey(localRoot)
	st, _ := LoadState(statePath, localRoot)

	if cfgIsRepo != nil {
		repoRootCache[key] = *cfgIsRepo
		if st.IsRepoRoot == nil {
			b := *cfgIsRepo
			st.IsRepoRoot = &b
			_ = SaveState(statePath, st)
		}
		return
	}
	if st.IsRepoRoot != nil {
		repoRootCache[key] = *st.IsRepoRoot
		return
	}
	if v, ok := repoRootCache[key]; ok {
		if st.IsRepoRoot == nil {
			b := v
			st.IsRepoRoot = &b
			_ = SaveState(statePath, st)
		}
		return
	}
	if cfapiProviderActive() {
		repoRootCache[key] = false
		return
	}
	v := IsLikelyRepoRoot(localRoot)
	repoRootCache[key] = v
	b := v
	st.IsRepoRoot = &b
	_ = SaveState(statePath, st)
}

func normalizeRootKey(localRoot string) string {
	abs := localRoot
	if filepath.IsAbs(localRoot) {
		abs = filepath.Clean(localRoot)
	} else if cleaned, err := filepath.Abs(localRoot); err == nil {
		abs = cleaned
	}
	return strings.ToLower(filepath.ToSlash(abs))
}
