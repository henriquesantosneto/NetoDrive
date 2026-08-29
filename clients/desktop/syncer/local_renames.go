package syncer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type localRename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func pendingLocalRenamesPath(localRoot string) string {
	return filepath.Join(localChangesRoot(localRoot), "pending-renames.jsonl")
}

// EnqueueLocalRename records an Explorer rename (CFAPI NOTIFY_RENAME).
func EnqueueLocalRename(localRoot, from, to string) error {
	from = filepath.ToSlash(strings.Trim(from, "/"))
	to = filepath.ToSlash(strings.Trim(to, "/"))
	if from == "" || to == "" || from == to {
		return nil
	}
	path := pendingLocalRenamesPath(localRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, _ := PendingLocalRenameSet(localRoot)
	if _, ok := existing[from+"->"+to]; ok {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(localRename{From: from, To: to})
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func PendingLocalRenameSet(localRoot string) (map[string]localRename, error) {
	out := map[string]localRename{}
	path := pendingLocalRenamesPath(localRoot)
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
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rn localRename
		if err := json.Unmarshal([]byte(line), &rn); err != nil {
			continue
		}
		rn.From = filepath.ToSlash(strings.Trim(rn.From, "/"))
		rn.To = filepath.ToSlash(strings.Trim(rn.To, "/"))
		if rn.From == "" || rn.To == "" {
			continue
		}
		out[rn.From+"->"+rn.To] = rn
	}
	return out, sc.Err()
}

func ClearLocalRename(localRoot string, rn localRename) error {
	set, err := PendingLocalRenameSet(localRoot)
	if err != nil {
		return err
	}
	key := rn.From + "->" + rn.To
	if _, ok := set[key]; !ok {
		return nil
	}
	delete(set, key)
	return rewritePendingRenames(localRoot, set)
}

func rewritePendingRenames(localRoot string, set map[string]localRename) error {
	path := pendingLocalRenamesPath(localRoot)
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
	for _, rn := range set {
		line, err := json.Marshal(rn)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func movePlaceholderMeta(localRoot, from, to string) error {
	fromPath := metaSidecarPath(localRoot, from)
	toPath := metaSidecarPath(localRoot, to)
	if _, err := os.Stat(fromPath); err != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(toPath), 0o755); err != nil {
		return err
	}
	if err := os.Rename(fromPath, toPath); err != nil {
		return err
	}
	return nil
}

func migratePinnedPaths(pinned []string, from, to string) []string {
	from = filepath.ToSlash(strings.Trim(from, "/"))
	to = filepath.ToSlash(strings.Trim(to, "/"))
	out := make([]string, 0, len(pinned))
	for _, p := range pinned {
		p = filepath.ToSlash(strings.Trim(p, "/"))
		switch {
		case p == from:
			out = append(out, to)
		case strings.HasPrefix(p, from+"/"):
			out = append(out, to+p[len(from):])
		default:
			out = append(out, p)
		}
	}
	return out
}

// applyPendingLocalRenames moves server-side paths for Explorer renames queued by the provider.
func applyPendingLocalRenames(c *Client, localRoot, remotePrefix string, legacyRemotes map[string]string, st *SyncState) error {
	set, err := PendingLocalRenameSet(localRoot)
	if err != nil || len(set) == 0 {
		return err
	}
	for _, rn := range set {
		from, to := rn.From, rn.To
		_ = movePlaceholderMeta(localRoot, from, to)
		st.Pinned = migratePinnedPaths(st.Pinned, from, to)
		if h, ok := st.Known[from]; ok {
			st.Known[to] = h
			delete(st.Known, from)
		}
		if e, ok := st.Entries[from]; ok {
			st.Entries[to] = e
			delete(st.Entries, from)
		}

		localPath := filepath.Join(localRoot, filepath.FromSlash(to))
		newRemote := to
		if remotePrefix != "" {
			newRemote = remotePrefix + "/" + to
		}
		oldRemote := remoteDeletePath(from, remotePrefix, legacyRemotes)

		if _, err := os.Stat(localPath); err == nil {
			syncLog("↪ rename upload %s -> %s", from, to)
			if _, err := c.Upload(localPath, newRemote); err != nil {
				return fmt.Errorf("rename upload %s: %w", to, err)
			}
		}
		syncLog("↪ rename delete remote %s", oldRemote)
		if err := c.Delete(oldRemote); err != nil {
			fmt.Fprintf(os.Stderr, "aviso: rename delete %s: %v\n", oldRemote, err)
		}
		if oldRemote != from {
			_ = c.Delete(from)
		}
		if err := ClearLocalRename(localRoot, rn); err != nil {
			return err
		}
	}
	return nil
}
