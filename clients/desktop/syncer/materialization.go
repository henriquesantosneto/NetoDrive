package syncer

import "os"

// localFilePresentInSyncRoot reports whether a non-directory file exists in the sync root.
func localFilePresentInSyncRoot(localRoot, rel string) bool {
	path := placeholderPath(localRoot, rel)
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !st.IsDir()
}

func indexMetaStorePresent(localRoot string) map[string]string {
	all := indexMetaStore(localRoot)
	if len(all) == 0 {
		return all
	}
	out := make(map[string]string, len(all))
	for rel := range all {
		if h, ok := indexLocalFileHash(localRoot, rel); ok {
			out[rel] = h
		}
	}
	return out
}

// remoteFilesNeedMaterialization reports remote files that still need placeholder creation.
func remoteFilesNeedMaterialization(localRoot string, man *Manifest) bool {
	if man == nil {
		return false
	}
	pending, _ := PendingLocalDeleteSet(localRoot)
	for _, e := range man.Files {
		if e.IsDir {
			continue
		}
		rel, _ := localRelFromRemote(e.Path)
		if rel == "" {
			continue
		}
		if pending[rel] {
			continue
		}
		if localFilePresentInSyncRoot(localRoot, rel) {
			continue
		}
		if shouldRematerializePlaceholder(localRoot, rel, pending) {
			return true
		}
	}
	return false
}
