package syncer

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func localChangesRoot(localRoot string) string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "NetoDrive", "local-changes", syncRootDataID(localRoot))
}

func pendingLocalDeletesPath(localRoot string) string {
	return filepath.Join(localChangesRoot(localRoot), "pending-deletes.txt")
}

// EnqueueLocalDelete records a user delete under CFAPI (Explorer NOTIFY_DELETE).
func EnqueueLocalDelete(localRoot, rel string) error {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	if rel == "" {
		return nil
	}
	path := pendingLocalDeletesPath(localRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, _ := PendingLocalDeleteSet(localRoot)
	if existing[rel] {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(rel + "\n")
	return err
}

func PendingLocalDeleteSet(localRoot string) (map[string]bool, error) {
	out := map[string]bool{}
	path := pendingLocalDeletesPath(localRoot)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		rel := filepath.ToSlash(strings.Trim(sc.Text(), "/"))
		if rel != "" {
			out[rel] = true
		}
	}
	return out, sc.Err()
}

func HasPendingLocalChanges(localRoot string) bool {
	if set, err := PendingLocalDeleteSet(localRoot); err == nil && len(set) > 0 {
		return true
	}
	if set, err := PendingLocalModifySet(localRoot); err == nil && len(set) > 0 {
		return true
	}
	if set, err := PendingLocalRenameSet(localRoot); err == nil && len(set) > 0 {
		return true
	}
	if _, err := os.Stat(pendingPinOpsPath(localRoot)); err == nil {
		return true
	}
	return false
}

func ClearLocalDelete(localRoot, rel string) error {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	set, err := PendingLocalDeleteSet(localRoot)
	if err != nil {
		return err
	}
	if !set[rel] {
		return nil
	}
	delete(set, rel)
	return rewritePendingDeletes(localRoot, set)
}

func ClearAllLocalDeletes(localRoot string) error {
	path := pendingLocalDeletesPath(localRoot)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func rewritePendingDeletes(localRoot string, set map[string]bool) error {
	path := pendingLocalDeletesPath(localRoot)
	if len(set) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for rel := range set {
		b.WriteString(rel)
		b.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func filterKnownExcludingDeletes(localRoot string, known map[string]string) map[string]string {
	if len(known) == 0 {
		return known
	}
	pending, err := PendingLocalDeleteSet(localRoot)
	if err != nil || len(pending) == 0 {
		return known
	}
	out := make(map[string]string, len(known))
	for rel, hash := range known {
		if pending[rel] {
			continue
		}
		out[rel] = hash
	}
	return out
}
