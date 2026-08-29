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

// SyncFolder uploads local changes and downloads remote-only files into localRoot.
// All paths map to the account root tree (no per-device prefix).
func SyncFolder(c *Client, localRoot string) error {
	return syncFolder(c, localRoot, "")
}

func syncFolder(c *Client, localRoot, remotePrefix string) error {
	localRoot, err := filepath.Abs(localRoot)
	if err != nil {
		return err
	}
	remotePrefix = strings.Trim(remotePrefix, "/")

	local := map[string]string{} // relative -> hash
	err = filepath.Walk(localRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(localRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(filepath.Base(rel), ".") {
			return nil
		}
		hash, _, err := FileHash(path)
		if err != nil {
			return err
		}
		local[rel] = hash
		return nil
	})
	if err != nil {
		return err
	}

	man, err := c.Manifest()
	if err != nil {
		return err
	}
	remote := map[string]ManifestEntry{}
	for _, e := range man.Files {
		if e.IsDir {
			continue
		}
		rel := e.Path
		if remotePrefix != "" {
			if !strings.HasPrefix(e.Path, remotePrefix+"/") && e.Path != remotePrefix {
				continue
			}
			rel = strings.TrimPrefix(e.Path, remotePrefix+"/")
			if e.Path == remotePrefix {
				continue
			}
		}
		remote[rel] = e
	}

	// Upload local newer/missing
	for rel, hash := range local {
		re, ok := remote[rel]
		if ok && re.Hash == hash {
			continue
		}
		remotePath := rel
		if remotePrefix != "" {
			remotePath = remotePrefix + "/" + rel
		}
		localPath := filepath.Join(localRoot, filepath.FromSlash(rel))
		fmt.Printf("↑ %s\n", remotePath)
		if _, err := c.Upload(localPath, remotePath); err != nil {
			return fmt.Errorf("upload %s: %w", remotePath, err)
		}
	}

	// Download remote-only
	for rel, e := range remote {
		if _, ok := local[rel]; ok {
			continue
		}
		localPath := filepath.Join(localRoot, filepath.FromSlash(rel))
		fmt.Printf("↓ %s\n", e.Path)
		if err := c.Download(e.Path, localPath); err != nil {
			return fmt.Errorf("download %s: %w", e.Path, err)
		}
	}
	return nil
}
