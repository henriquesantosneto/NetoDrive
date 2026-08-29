package syncer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Change struct {
	ID     int64  `json:"id"`
	Action string `json:"action"`
	File   struct {
		Path     string `json:"path"`
		DeviceID string `json:"device_id"`
		Deleted  bool   `json:"deleted"`
	} `json:"file"`
}

type ChangesResponse struct {
	Changes []Change `json:"changes"`
	Cursor  int64    `json:"cursor"`
}

func (c *Client) FetchChanges(since int64) (*ChangesResponse, error) {
	req, err := c.authReq(http.MethodGet, fmt.Sprintf("/api/sync/changes?since=%d&limit=1000", since), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("changes status %d: %s", res.StatusCode, string(b))
	}
	var out ChangesResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func applyRemoteChanges(c *Client, localRoot string, cursor int64, st *SyncState) (int64, error) {
	const maxRounds = 100
	for round := 0; round < maxRounds; round++ {
		resp, err := c.FetchChanges(cursor)
		if err != nil {
			return cursor, err
		}
		if len(resp.Changes) == 0 {
			return cursor, nil
		}
		for _, ch := range resp.Changes {
			if ch.Action != "delete" {
				continue
			}
			if ch.File.DeviceID == c.DeviceID {
				continue
			}
			rel, _ := localRelFromRemote(ch.File.Path)
			if rel == "" {
				continue
			}
			if st != nil {
				_ = CancelPendingRenamesForDeletedRemote(localRoot, st, rel)
			}
			fmt.Printf("× local %s (excluido no servidor)\n", rel)
			if err := deleteLocalFile(localRoot, rel); err != nil {
				fmt.Fprintf(os.Stderr, "aviso: nao foi possivel remover %s: %v\n", rel, err)
				continue
			}
		}
		if resp.Cursor <= cursor {
			return resp.Cursor, nil
		}
		cursor = resp.Cursor
	}
	return cursor, fmt.Errorf("change feed exceeded %d rounds; tente novamente", maxRounds)
}

func deleteLocalFile(localRoot, rel string) error {
	if rel == "" {
		return nil
	}
	removePlatformPlaceholder(localRoot, rel)
	_ = deleteLocalFilePlatform(localRoot, rel)
	abs := filepath.Join(localRoot, filepath.FromSlash(rel))
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		_ = deleteLocalFilePlatform(localRoot, rel)
		if err2 := os.Remove(abs); err2 != nil && !os.IsNotExist(err) {
			return err2
		}
	}
	// Remove legacy PC/Android copy if still on disk.
	for _, prefix := range legacyDevicePrefixes {
		legacy := filepath.Join(localRoot, prefix, filepath.FromSlash(rel))
		if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	pruneEmptyDirs(localRoot, filepath.Dir(abs))
	for _, prefix := range legacyDevicePrefixes {
		pruneEmptyDirs(localRoot, filepath.Join(localRoot, prefix))
	}
	return nil
}

func remoteDeletePath(rel, remotePrefix string, legacyRemotes map[string]string) string {
	if legacy, ok := legacyRemotes[rel]; ok && legacy != "" {
		return legacy
	}
	if remotePrefix != "" {
		return remotePrefix + "/" + rel
	}
	return rel
}

func pruneEmptyDirs(localRoot, dir string) {
	localRoot = filepath.Clean(localRoot)
	dir = filepath.Clean(dir)
	for {
		if dir == localRoot || !strings.HasPrefix(dir, localRoot+string(os.PathSeparator)) {
			if dir == localRoot {
				break
			}
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func scanLocalFiles(localRoot string) (map[string]string, error) {
	local := map[string]string{}
	err := filepath.Walk(localRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if shouldSkipWalkEntry(localRoot, path, info.Name(), info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(localRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasSuffix(strings.ToLower(rel), ".lnk") && isPlatformPlaceholder(path) {
			rel = strings.TrimSuffix(rel, ".lnk")
		}
		absRel := rel
		isLegacy := false
		for _, prefix := range legacyDevicePrefixes {
			if absRel == prefix || strings.HasPrefix(absRel, prefix+"/") {
				isLegacy = true
				break
			}
		}
		rel = localRelFromLocal(rel)
		if rel == "" {
			return nil
		}
		if isLegacy {
			if _, exists := local[rel]; exists {
				return nil
			}
		}
		var hash string
		if meta, ok := readPlaceholderMetaForPath(localRoot, path, rel); ok {
			hash = meta.Hash
		} else if isPlatformPlaceholder(path) {
			// Cloud placeholder without sidecar — do not read file (can hang hydrating offline).
			return nil
		} else {
			hash, _, err = FileHash(path)
			if err != nil {
				return err
			}
		}
		local[rel] = hash
		return nil
	})
	return local, err
}
