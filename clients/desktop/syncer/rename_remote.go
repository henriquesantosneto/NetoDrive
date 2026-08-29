package syncer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var ErrRenameAPINotSupported = errors.New("rename api not supported on server")

func (c *Client) Rename(oldPath, newPath string) error {
	oldPath = strings.Trim(strings.ReplaceAll(oldPath, "\\", "/"), "/")
	newPath = strings.Trim(strings.ReplaceAll(newPath, "\\", "/"), "/")
	if oldPath == "" || newPath == "" {
		return fmt.Errorf("invalid rename paths")
	}
	body, err := json.Marshal(map[string]string{"from": oldPath, "to": newPath})
	if err != nil {
		return err
	}
	req, err := c.authReq(http.MethodPost, "/api/sync/rename", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return ErrRenameAPINotSupported
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("rename failed: %s", string(b))
	}
	return nil
}

func (c *Client) remotePathActive(remotePath string) (bool, error) {
	man, err := c.Manifest()
	if err != nil {
		return false, err
	}
	return manifestHasPath(man, remotePath), nil
}

func renameRemotePaths(c *Client, man *Manifest, from, to, remotePrefix string, legacyRemotes map[string]string) error {
	oldRemote := remoteDeletePath(from, remotePrefix, legacyRemotes)
	newRemote := to
	if remotePrefix != "" {
		newRemote = remotePrefix + "/" + to
	}

	if manifestHasPath(man, newRemote) {
		syncLog("↪ rename ja aplicado: %s existe no servidor", newRemote)
		_ = c.deleteRemoteIgnoreMissing(oldRemote)
		if oldRemote != from {
			_ = c.deleteRemoteIgnoreMissing(from)
		}
		return nil
	}

	syncLog("↪ rename remoto %s -> %s", oldRemote, newRemote)
	if err := c.Rename(oldRemote, newRemote); err == nil {
		return nil
	} else if !errors.Is(err, ErrRenameAPINotSupported) {
		if leg, ok := legacyRemotes[from]; ok && leg != oldRemote {
			syncLog("↪ rename remoto (legacy) %s -> %s", leg, newRemote)
			if err2 := c.Rename(leg, newRemote); err2 == nil {
				return nil
			}
		}
		if manifestHasPath(man, newRemote) {
			_ = c.deleteRemoteIgnoreMissing(oldRemote)
			return nil
		}
		return err
	}

	syncLog("↪ rename via stream (servidor sem /api/sync/rename)")
	if err := c.renameViaStream(oldRemote, newRemote); err == nil {
		return nil
	} else if leg, ok := legacyRemotes[from]; ok && leg != oldRemote {
		if err2 := c.renameViaStream(leg, newRemote); err2 == nil {
			return nil
		}
	}

	return fmt.Errorf("%w: atualize e reinicie o servidor NetoDrive", ErrRenameAPINotSupported)
}

func (c *Client) deleteRemoteIgnoreMissing(remotePath string) error {
	err := c.Delete(remotePath)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") ||
		strings.Contains(err.Error(), "404") {
		return nil
	}
	return err
}

func (c *Client) renameViaStream(oldRemote, newRemote string) error {
	req, err := c.authReq(http.MethodGet, "/api/sync/download/"+escapePath(oldRemote), nil)
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
		return fmt.Errorf("rename stream download failed: %s", string(b))
	}
	size := res.ContentLength
	if size < 0 {
		size = 0
	}
	if _, err := c.uploadStream(res.Body, size, newRemote, time.Now().UTC()); err != nil {
		return err
	}
	if err := c.deleteRemoteIgnoreMissing(oldRemote); err != nil {
		fmt.Fprintf(os.Stderr, "aviso: rename remove %s: %v\n", oldRemote, err)
	}
	return nil
}

func (c *Client) uploadStream(r io.Reader, size int64, remotePath string, modTime time.Time) (*FileMeta, error) {
	req, err := c.authReq(http.MethodPut, "/api/sync/upload", r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-File-Path", remotePath)
	req.Header.Set("X-Device-Id", c.DeviceID)
	req.Header.Set("X-File-Mtime", modTime.Format(time.RFC3339Nano))
	req.Header.Set("Content-Type", "application/octet-stream")
	if size > 0 {
		req.ContentLength = size
	}
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

func renameLocalState(localRoot string, st *SyncState, from, to string) {
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
}
