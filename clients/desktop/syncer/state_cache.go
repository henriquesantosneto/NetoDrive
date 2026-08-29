package syncer

import (
	"sync"
)

var (
	stateCacheMu sync.RWMutex
	stateCache   = map[string]SyncState{}
)

func LoadStateCached(path, localFolder string) (SyncState, error) {
	stateCacheMu.RLock()
	if st, ok := stateCache[path]; ok {
		stateCacheMu.RUnlock()
		if localFolder != "" {
			st.LocalFolder = localFolder
		}
		return st, nil
	}
	stateCacheMu.RUnlock()

	st, err := LoadState(path, localFolder)
	if err != nil {
		return st, err
	}
	stateCacheMu.Lock()
	stateCache[path] = st
	stateCacheMu.Unlock()
	return st, nil
}

func SaveStateCached(path string, st SyncState) error {
	if err := SaveState(path, st); err != nil {
		return err
	}
	stateCacheMu.Lock()
	stateCache[path] = st
	stateCacheMu.Unlock()
	return nil
}

func InvalidateStateCache(path string) {
	stateCacheMu.Lock()
	delete(stateCache, path)
	stateCacheMu.Unlock()
}
