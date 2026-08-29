package syncer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	BaseURL  string
	Token    string
	DeviceID string
	HTTP     *http.Client
}

type ManifestEntry struct {
	Path    string    `json:"path"`
	Hash    string    `json:"hash"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	MTime   time.Time `json:"mtime"`
	Version int64     `json:"version"`
}

type Manifest struct {
	Version int64           `json:"version"`
	Files   []ManifestEntry `json:"files"`
}

type FileMeta struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

func NewClient(baseURL, token, deviceID string) *Client {
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Token:    token,
		DeviceID: deviceID,
		HTTP:     &http.Client{Timeout: 0},
	}
}

func (c *Client) Login(username, password string) error {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/auth/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("login failed: %s", string(b))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return err
	}
	c.Token = out.Token
	return nil
}

func (c *Client) Manifest() (*Manifest, error) {
	req, err := c.authReq(http.MethodGet, "/api/sync/manifest", nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest status %d", res.StatusCode)
	}
	var m Manifest
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (c *Client) Upload(localPath, remotePath string) (*FileMeta, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	req, err := c.authReq(http.MethodPut, "/api/sync/upload", f)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-File-Path", remotePath)
	req.Header.Set("X-Device-Id", c.DeviceID)
	req.Header.Set("X-File-Mtime", st.ModTime().UTC().Format(time.RFC3339Nano))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = st.Size()

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("upload failed: %s", string(b))
	}
	var meta FileMeta
	if err := json.NewDecoder(res.Body).Decode(&meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (c *Client) Download(remotePath, localPath string) error {
	req, err := c.authReq(http.MethodGet, "/api/sync/download/"+escapePath(remotePath), nil)
	if err != nil {
		return err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("download failed: %s", string(b))
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	tmp := localPath + ".partial"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, res.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, localPath)
}

func (c *Client) Delete(remotePath string) error {
	req, err := c.authReq(http.MethodDelete, "/api/files/"+escapePath(remotePath), nil)
	if err != nil {
		return err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("delete failed: %s", string(b))
	}
	return nil
}

func (c *Client) authReq(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	return req, nil
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func FileHash(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// SyncFolder mirrors localRoot with the account tree (placeholders when onDemand is true).
func SyncFolder(c *Client, localRoot, statePath string, onDemand bool) error {
	return syncFolder(c, localRoot, statePath, onDemand, "")
}

func syncFolder(c *Client, localRoot, statePath string, onDemand bool, remotePrefix string) error {
	localRoot, err := filepath.Abs(localRoot)
	if err != nil {
		return err
	}
	remotePrefix = strings.Trim(remotePrefix, "/")

	st, err := LoadState(statePath, localRoot)
	if err != nil {
		return err
	}
	st.OnDemand = onDemand

	newCursor, err := applyRemoteChanges(c, localRoot, st.ChangeCursor)
	if err != nil {
		return fmt.Errorf("apply changes: %w", err)
	}
	st.ChangeCursor = newCursor

	local, err := scanLocalFiles(localRoot)
	if err != nil {
		return err
	}

	man, err := c.Manifest()
	if err != nil {
		return err
	}
	remote := map[string]ManifestEntry{}
	legacyRemotes := map[string]string{}
	for _, e := range man.Files {
		if e.IsDir {
			continue
		}
		path := e.Path
		if remotePrefix != "" {
			if !strings.HasPrefix(path, remotePrefix+"/") && path != remotePrefix {
				continue
			}
			path = strings.TrimPrefix(path, remotePrefix+"/")
			if e.Path == remotePrefix {
				continue
			}
		}
		rel, legacyRemote := localRelFromRemote(path)
		if rel == "" {
			continue
		}
		if legacyRemote != "" {
			legacyRemotes[rel] = e.Path
		}
		remote[rel] = e
	}

	remoteHashes := map[string]string{}
	for rel, e := range remote {
		remoteHashes[rel] = e.Hash
	}

	plan := planSync(local, remoteHashes, st.Known)

	for _, rel := range plan.deleteLocal {
		fmt.Printf("× local %s (removido na web)\n", rel)
		if err := deleteLocalFile(localRoot, rel); err != nil {
			return fmt.Errorf("delete local %s: %w", rel, err)
		}
		delete(local, rel)
	}

	for _, rel := range plan.deleteRemote {
		remotePath := rel
		if remotePrefix != "" {
			remotePath = remotePrefix + "/" + rel
		}
		fmt.Printf("× remoto %s (removido neste PC)\n", remotePath)
		if err := c.Delete(remotePath); err != nil {
			return fmt.Errorf("delete remote %s: %w", remotePath, err)
		}
		delete(remote, rel)
	}

	for _, rel := range plan.upload {
		re, ok := remote[rel]
		localPath := filepath.Join(localRoot, filepath.FromSlash(rel))
		if ok && re.Hash == local[rel] {
			continue
		}
		if IsPlaceholderFile(localPath) {
			if meta, ok := readPlaceholderMeta(localPath); ok {
				if re.Hash == meta.Hash {
					continue
				}
			}
		}
		remotePath := rel
		if remotePrefix != "" {
			remotePath = remotePrefix + "/" + rel
		}
		fmt.Printf("↑ %s\n", remotePath)
		if _, err := c.Upload(localPath, remotePath); err != nil {
			return fmt.Errorf("upload %s: %w", remotePath, err)
		}
		if oldRemote, ok := legacyRemotes[rel]; ok && oldRemote != remotePath {
			fmt.Printf("↺ remove legacy %s\n", oldRemote)
			_ = c.Delete(oldRemote)
		}
	}

	for _, rel := range plan.download {
		e, ok := remote[rel]
		if !ok {
			continue
		}
		localPath := filepath.Join(localRoot, filepath.FromSlash(rel))
		downloadPath := e.Path
		if legacyRemote, ok := legacyRemotes[rel]; ok {
			downloadPath = legacyRemote
		}
		if st.OnDemand && !isPinnedPath(st.Pinned, rel) {
			fmt.Printf("☁ placeholder %s\n", rel)
			if err := writePlaceholder(localRoot, rel, placeholderMeta{Hash: e.Hash, Size: e.Size}); err != nil {
				return fmt.Errorf("placeholder %s: %w", rel, err)
			}
			continue
		}
		fmt.Printf("↓ %s\n", downloadPath)
		if err := c.Download(downloadPath, localPath); err != nil {
			return fmt.Errorf("download %s: %w", downloadPath, err)
		}
		if legacyRemote, ok := legacyRemotes[rel]; ok {
			fmt.Printf("↺ remove legacy %s\n", legacyRemote)
			_ = c.Delete(legacyRemote)
		}
	}

	// Download pinned paths that are still placeholders.
	if err := hydratePinnedFromManifest(c, localRoot, &st, remote, legacyRemotes); err != nil {
		return err
	}

	removeEmptyLegacyDirs(localRoot)

	local, err = scanLocalFiles(localRoot)
	if err != nil {
		return err
	}
	st.Known = local
	st.Entries = rebuildEntries(localRoot, local, remote, st)

	newCursor, err = applyRemoteChanges(c, localRoot, st.ChangeCursor)
	if err != nil {
		return fmt.Errorf("apply changes after sync: %w", err)
	}
	st.ChangeCursor = newCursor

	return SaveState(statePath, st)
}

func hydratePinnedFromManifest(c *Client, localRoot string, st *SyncState, remote map[string]ManifestEntry, legacyRemotes map[string]string) error {
	for rel, e := range remote {
		if !isPinnedPath(st.Pinned, rel) {
			continue
		}
		localPath := placeholderPath(localRoot, rel)
		if !IsPlaceholderFile(localPath) {
			if h, _, err := FileHash(localPath); err == nil && h == e.Hash {
				continue
			}
		}
		downloadPath := e.Path
		if legacyRemote, ok := legacyRemotes[rel]; ok {
			downloadPath = legacyRemote
		}
		fmt.Printf("↓ pinned %s\n", rel)
		if err := c.Download(downloadPath, localPath); err != nil {
			return fmt.Errorf("download pinned %s: %w", rel, err)
		}
	}
	return nil
}

func rebuildEntries(localRoot string, local map[string]string, remote map[string]ManifestEntry, st SyncState) map[string]FileEntry {
	entries := map[string]FileEntry{}
	for rel, hash := range local {
		entry := FileEntry{Hash: hash, Availability: AvHydrated}
		if isPinnedPath(st.Pinned, rel) {
			entry.Availability = AvPinned
		} else if IsPlaceholderFile(placeholderPath(localRoot, rel)) {
			entry.Availability = AvPlaceholder
		}
		if re, ok := remote[rel]; ok {
			entry.Size = re.Size
		}
		entries[rel] = entry
	}
	return entries
}
