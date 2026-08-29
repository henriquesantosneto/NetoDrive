package syncer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func writePlaceholderMeta(localRoot, rel string, meta placeholderMeta) error {
	migrateLegacyMetaSidecar(localRoot, rel)
	path := metaSidecarPath(localRoot, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func readPlaceholderMetaForRel(localRoot, rel string) (placeholderMeta, bool) {
	migrateLegacyMetaSidecar(localRoot, rel)
	path := metaSidecarPath(localRoot, rel)
	b, err := os.ReadFile(path)
	if err != nil {
		b, err = os.ReadFile(legacyMetaSidecarPath(localRoot, rel))
		if err != nil {
			return placeholderMeta{}, false
		}
	}
	var m placeholderMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return placeholderMeta{}, false
	}
	return m, true
}

func removePlaceholderMeta(localRoot, rel string) {
	_ = os.Remove(metaSidecarPath(localRoot, rel))
	_ = os.Remove(legacyMetaSidecarPath(localRoot, rel))
}

func indexMetaStore(localRoot string) map[string]string {
	out := map[string]string{}
	store := metaStoreRoot(localRoot)
	entries, err := os.ReadDir(store)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		rel := metaRelFromKey(strings.TrimSuffix(e.Name(), ".json"))
		if meta, ok := readPlaceholderMetaForRel(localRoot, rel); ok {
			out[rel] = meta.Hash
		}
	}
	return out
}
