package syncer

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type placeholderQueueEntry struct {
	Rel  string `json:"rel"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

func placeholderQueueRoot(localRoot string) string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "NetoDrive", "placeholder-queue", syncRootDataID(localRoot))
}

func placeholderQueuePath(localRoot string) string {
	return filepath.Join(placeholderQueueRoot(localRoot), "pending.jsonl")
}

func enqueuePlaceholder(localRoot, rel string, meta placeholderMeta) error {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	if rel == "" {
		return nil
	}
	if isPlaceholderQueued(localRoot, rel, meta.Hash) {
		return nil
	}
	path := placeholderQueuePath(localRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	entry := placeholderQueueEntry{Rel: rel, Hash: meta.Hash, Size: meta.Size}
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func isPlaceholderQueued(localRoot, rel, hash string) bool {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	path := placeholderQueuePath(localRoot)
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var entry placeholderQueueEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if filepath.ToSlash(strings.Trim(entry.Rel, "/")) == rel && entry.Hash == hash {
			return true
		}
	}
	return false
}
