package syncer

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type FileAvailability string

const (
	AvPlaceholder FileAvailability = "placeholder"
	AvHydrated    FileAvailability = "hydrated"
	AvPinned      FileAvailability = "pinned"
)

type FileEntry struct {
	Hash         string           `json:"hash"`
	Size         int64            `json:"size,omitempty"`
	Availability FileAvailability `json:"availability"`
}

// SyncState tracks sync snapshot, placeholders and pinned paths.
type SyncState struct {
	LocalFolder  string               `json:"local_folder"`
	IsRepoRoot   *bool                `json:"is_repo_root,omitempty"`
	ChangeCursor int64                `json:"change_cursor"`
	OnDemand     bool                 `json:"on_demand"`
	Pinned           []string             `json:"pinned,omitempty"`
	Entries          map[string]FileEntry `json:"entries,omitempty"`
	Known            map[string]string    `json:"known,omitempty"` // legacy: path -> hash
	LastManifestFP   string               `json:"last_manifest_fp,omitempty"`
}

func DefaultStatePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "sync-state.json")
}

func LoadState(path, localFolder string) (SyncState, error) {
	st := SyncState{
		LocalFolder: localFolder,
		OnDemand:    true,
		Known:       map[string]string{},
		Entries:     map[string]FileEntry{},
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return SyncState{LocalFolder: localFolder, OnDemand: true, Known: map[string]string{}, Entries: map[string]FileEntry{}}, nil
	}
	if st.Known == nil {
		st.Known = map[string]string{}
	}
	if st.Entries == nil {
		st.Entries = map[string]FileEntry{}
	}
	// Migrate legacy known map into entries.
	for rel, hash := range st.Known {
		if _, ok := st.Entries[rel]; !ok {
			st.Entries[rel] = FileEntry{Hash: hash, Availability: AvHydrated}
		}
	}
	if localFolder != "" && st.LocalFolder != "" && st.LocalFolder != localFolder {
		st.LocalFolder = localFolder
		st.ChangeCursor = 0
		st.Known = map[string]string{}
		st.Entries = map[string]FileEntry{}
	} else if localFolder != "" {
		st.LocalFolder = localFolder
	}
	return st, nil
}

func SaveState(path string, st SyncState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Keep known in sync for older readers.
	if st.Known == nil {
		st.Known = map[string]string{}
	}
	for rel, entry := range st.Entries {
		st.Known[rel] = entry.Hash
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
