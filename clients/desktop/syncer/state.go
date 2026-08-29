package syncer

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SyncState tracks the last synced snapshot for delete propagation.
type SyncState struct {
	LocalFolder  string            `json:"local_folder"`
	ChangeCursor int64             `json:"change_cursor"`
	Known        map[string]string `json:"known"` // relative path -> content hash
}

func DefaultStatePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "sync-state.json")
}

func LoadState(path, localFolder string) (SyncState, error) {
	st := SyncState{
		LocalFolder: localFolder,
		Known:       map[string]string{},
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return SyncState{LocalFolder: localFolder, Known: map[string]string{}}, nil
	}
	if st.Known == nil {
		st.Known = map[string]string{}
	}
	if st.LocalFolder != localFolder {
		st.LocalFolder = localFolder
		st.ChangeCursor = 0
		st.Known = map[string]string{}
	}
	return st, nil
}

func SaveState(path string, st SyncState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
