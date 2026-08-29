package syncer

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func pendingLocalModifiesPath(localRoot string) string {
	return filepath.Join(localChangesRoot(localRoot), "pending-modifies.txt")
}

// EnqueueLocalModify records a user edit under CFAPI (Explorer file close).
func EnqueueLocalModify(localRoot, rel string) error {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	if rel == "" {
		return nil
	}
	path := pendingLocalModifiesPath(localRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, _ := PendingLocalModifySet(localRoot)
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

func PendingLocalModifySet(localRoot string) (map[string]bool, error) {
	out := map[string]bool{}
	path := pendingLocalModifiesPath(localRoot)
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

func ClearLocalModify(localRoot, rel string) error {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	set, err := PendingLocalModifySet(localRoot)
	if err != nil {
		return err
	}
	if !set[rel] {
		return nil
	}
	delete(set, rel)
	return rewritePendingModifies(localRoot, set)
}

func rewritePendingModifies(localRoot string, set map[string]bool) error {
	path := pendingLocalModifiesPath(localRoot)
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
