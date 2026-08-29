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

func isPlaceholderQueuedRel(localRoot, rel string) bool {
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
		if filepath.ToSlash(strings.Trim(entry.Rel, "/")) == rel {
			return true
		}
	}
	return false
}

func removePlaceholderQueueRel(localRoot, rel string) error {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	path := placeholderQueuePath(localRoot)
	lines, err := readPlaceholderQueueLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(lines) == 0 {
		return nil
	}
	var kept []string
	removed := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry placeholderQueueEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			kept = append(kept, line)
			continue
		}
		if filepath.ToSlash(strings.Trim(entry.Rel, "/")) == rel {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	if !removed {
		return nil
	}
	return rewritePlaceholderQueue(path, kept)
}

func readPlaceholderQueueLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, sc.Err()
}

func rewritePlaceholderQueue(path string, lines []string) error {
	if len(lines) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// shouldRematerializePlaceholder reports incomplete CFAPI placeholder creation.
func shouldRematerializePlaceholder(localRoot, rel string, pending map[string]bool) bool {
	if pending != nil && pending[rel] {
		return false
	}
	if localFilePresentInSyncRoot(localRoot, rel) {
		return false
	}
	if _, ok := readPlaceholderMetaForRel(localRoot, rel); ok {
		return true
	}
	return isPlaceholderQueuedRel(localRoot, rel)
}

func HasPendingPlaceholderQueue(localRoot string) bool {
	path := placeholderQueuePath(localRoot)
	lines, err := readPlaceholderQueueLines(path)
	return err == nil && len(lines) > 0
}
