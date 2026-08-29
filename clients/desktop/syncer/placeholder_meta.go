package syncer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func metaSidecarPath(localRoot, rel string) string {
	key := strings.ReplaceAll(filepath.ToSlash(rel), "/", "__")
	if key == "" {
		key = "_root"
	}
	return filepath.Join(localRoot, ".netodrive", "meta", key+".json")
}

func writePlaceholderMeta(localRoot, rel string, meta placeholderMeta) error {
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
	path := metaSidecarPath(localRoot, rel)
	b, err := os.ReadFile(path)
	if err != nil {
		return placeholderMeta{}, false
	}
	var meta placeholderMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		return placeholderMeta{}, false
	}
	return meta, true
}

func removePlaceholderMeta(localRoot, rel string) {
	_ = os.Remove(metaSidecarPath(localRoot, rel))
}
