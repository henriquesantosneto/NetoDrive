package syncer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var folderSyncMu sync.Mutex

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
		HTTP:     newSyncHTTPClient(),
	}
}

func newSyncHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ResponseHeaderTimeout: 2 * time.Minute,
			ExpectContinueTimeout: 1 * time.Second,
		},
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
	return c.uploadStream(f, st.Size(), remotePath, st.ModTime().UTC())
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
	if !folderSyncMu.TryLock() {
		return fmt.Errorf("sync interno ocupado (ciclo anterior ainda ativo)")
	}
	defer folderSyncMu.Unlock()

	syncLog("sync: iniciando...")
	SetPlaceholderBulkSync(true)
	defer SetPlaceholderBulkSync(false)
	return syncFolder(c, localRoot, statePath, onDemand, "")
}

func (c *Client) CreateRemoteDir(remotePath string) error {
	body, _ := json.Marshal(map[string]bool{"is_dir": true})
	req, err := c.authReq(http.MethodPost, "/api/files/"+escapePath(remotePath), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Id", c.DeviceID)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("mkdir failed: %s", string(b))
	}
	return nil
}

func manifestFingerprint(m *Manifest) string {
	if m == nil {
		return ""
	}
	h := sha256.New()
	fmt.Fprintf(h, "v%d|", m.Version)
	for _, e := range m.Files {
		if e.IsDir {
			fmt.Fprintf(h, "D:%s;", e.Path)
			continue
		}
		fmt.Fprintf(h, "%s:%s:%d;", e.Path, e.Hash, e.Size)
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:8])
}

func syncFolder(c *Client, localRoot, statePath string, onDemand bool, remotePrefix string) error {
	if !filepath.IsAbs(localRoot) {
		var err error
		localRoot, err = filepath.Abs(localRoot)
		if err != nil {
			return err
		}
	}
	remotePrefix = strings.Trim(remotePrefix, "/")

	st, err := LoadStateCached(statePath, localRoot)
	if err != nil {
		return err
	}
	st.OnDemand = onDemand
	setSyncWalkContext(localRoot, &st)

	syncLog("sync: verificando servidor...")
	if err := c.Ping(); err != nil {
		return fmt.Errorf("servidor indisponivel (%s): %w", c.BaseURL, err)
	}

	man, err := c.Manifest()
	if err != nil {
		return err
	}
	syncLog("sync: manifest com %d entradas", len(man.Files))
	fp := manifestFingerprint(man)
	ensureKnownDirs(&st)
	localDirsPeek, _ := scanLocalDirsForSync(localRoot)
	if fp != "" && fp == st.LastManifestFP && !HasPendingLocalChanges(localRoot) && !dirsChanged(localDirsPeek, st.KnownDirs) && !localContentChangedSinceSync(localRoot, st.Known) {
		if !remoteFilesNeedMaterialization(localRoot, man) {
			syncLog("sync: sem alteracoes remotas (skip scan CFAPI)")
			return SaveStateCached(statePath, st)
		}
		syncLog("sync: manifest igual mas arquivos remotos precisam materializar")
	}

	if HasPendingLocalChanges(localRoot) {
		if err := applyPendingPinOps(statePath, localRoot); err != nil {
			return err
		}
		syncLog("sync: aplicando renames locais pendentes...")
		if err := applyPendingLocalRenames(c, localRoot, remotePrefix, legacyRemotesFromManifest(man, remotePrefix), &st); err != nil {
			return err
		}
		syncLog("sync: aplicando deletes locais pendentes...")
		if err := applyPendingLocalDeletes(c, localRoot, remotePrefix, map[string]string{}, &st); err != nil {
			return err
		}
		man, err = c.Manifest()
		if err != nil {
			return err
		}
		fp = manifestFingerprint(man)
		syncLog("sync: manifest atualizado (%d entradas)", len(man.Files))
	}

	syncLog("sync: aplicando changes remotos...")
	newCursor, err := applyRemoteChanges(c, localRoot, st.ChangeCursor)
	if err != nil {
		if IsConnectionError(err) {
			fmt.Fprintf(os.Stderr, "aviso: feed de changes indisponivel: %v\n", err)
		} else {
			return fmt.Errorf("apply changes: %w", err)
		}
	} else {
		st.ChangeCursor = newCursor
	}

	remotePre := map[string]ManifestEntry{}
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
		rel, _ := localRelFromRemote(path)
		if rel == "" {
			continue
		}
		remotePre[rel] = e
	}
	if err := migrateLegacyPlaceholders(localRoot, remotePre, onDemand, st.Pinned); err != nil {
		return fmt.Errorf("migrate placeholders: %w", err)
	}

	syncLog("sync: escaneando pasta local...")
	knownForScan := filterKnownExcludingDeletes(localRoot, st.Known)
	local, err := scanLocalFilesForSync(localRoot, knownForScan)
	if err != nil {
		return err
	}
	localDirs, err := scanLocalDirsForSync(localRoot)
	if err != nil {
		return err
	}
	syncLog("sync: %d arquivos locais, %d pastas locais", len(local), len(localDirs))

	remote := map[string]ManifestEntry{}
	remoteDirs := map[string]bool{}
	legacyRemotes := map[string]string{}
	for _, e := range man.Files {
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
		if e.IsDir {
			remoteDirs[rel] = true
			continue
		}
		if legacyRemote != "" {
			legacyRemotes[rel] = e.Path
		}
		remote[rel] = e
	}

	dirPlan := planDirSync(localDirs, remoteDirs, st.KnownDirs)
	syncLog("sync: dirs upload=%d download=%d deleteLocal=%d deleteRemote=%d",
		len(dirPlan.upload), len(dirPlan.download), len(dirPlan.deleteLocal), len(dirPlan.deleteRemote))

	sortDirsByDepth(dirPlan.upload)
	for _, rel := range dirPlan.upload {
		remotePath := rel
		if remotePrefix != "" {
			remotePath = remotePrefix + "/" + rel
		}
		syncLog("↑ dir %s", remotePath)
		if err := c.CreateRemoteDir(remotePath); err != nil {
			return fmt.Errorf("mkdir remote %s: %w", remotePath, err)
		}
	}

	for _, rel := range dirPlan.download {
		syncLog("↓ dir %s", rel)
		if err := createLocalDir(localRoot, rel); err != nil {
			fmt.Fprintf(os.Stderr, "aviso: mkdir local %s: %v\n", rel, err)
		}
	}

	for _, rel := range dirPlan.deleteLocal {
		syncLog("× dir local %s (removido na web)", rel)
		if err := deleteLocalDir(localRoot, rel); err != nil {
			fmt.Fprintf(os.Stderr, "aviso: nao foi possivel remover pasta %s: %v\n", rel, err)
			continue
		}
		delete(localDirs, rel)
		delete(st.KnownDirs, rel)
	}

	for _, rel := range dirPlan.deleteRemote {
		remotePath := remoteDeletePath(rel, remotePrefix, nil)
		syncLog("× dir remoto %s (removido neste PC)", remotePath)
		if err := c.Delete(remotePath); err != nil {
			return fmt.Errorf("delete remote dir %s: %w", remotePath, err)
		}
		delete(remoteDirs, rel)
		delete(st.KnownDirs, rel)
	}

	remoteHashes := map[string]string{}
	for rel, e := range remote {
		remoteHashes[rel] = e.Hash
	}

	pendingDeletes, _ := PendingLocalDeleteSet(localRoot)
	pendingRenames := pendingRenameMap(localRoot)
	plan := planSync(local, remoteHashes, st.Known, PlanSyncOptions{
		LocalRoot:            localRoot,
		RematerializeMissing: cfapiProviderActive(),
		PendingLocalDeletes:  pendingDeletes,
		PendingLocalRenames:  pendingRenames,
	})
	syncLog("sync: plan upload=%d download=%d deleteLocal=%d deleteRemote=%d",
		len(plan.upload), len(plan.download), len(plan.deleteLocal), len(plan.deleteRemote))

	for _, rel := range plan.deleteLocal {
		syncLog("× local %s (removido na web)", rel)
		if err := deleteLocalFile(localRoot, rel); err != nil {
			fmt.Fprintf(os.Stderr, "aviso: nao foi possivel remover %s: %v\n", rel, err)
			continue
		}
		delete(local, rel)
		delete(st.Known, rel)
		delete(st.Entries, rel)
	}

	for _, rel := range plan.deleteRemote {
		remotePath := remoteDeletePath(rel, remotePrefix, legacyRemotes)
		syncLog("× remoto %s (removido neste PC)", remotePath)
		if err := c.Delete(remotePath); err != nil {
			return fmt.Errorf("delete remote %s: %w", remotePath, err)
		}
		// Best-effort: remove duplicate at root if legacy path differed.
		if remotePath != rel {
			_ = c.Delete(rel)
		}
		removePlatformPlaceholder(localRoot, rel)
		delete(remote, rel)
		delete(st.Known, rel)
		delete(st.Entries, rel)
		_ = ClearLocalDelete(localRoot, rel)
	}

	for _, rel := range plan.upload {
		re, ok := remote[rel]
		localPath := filepath.Join(localRoot, filepath.FromSlash(rel))
		if ok && re.Hash == local[rel] {
			continue
		}
		if isCloudOnlyPlaceholder(localRoot, rel) {
			if ok {
				syncLog("☁ atualiza placeholder %s", rel)
				if err := writePlaceholder(localRoot, rel, placeholderMeta{Hash: re.Hash, Size: re.Size}); err != nil {
					fmt.Fprintf(os.Stderr, "aviso: placeholder %s: %v\n", rel, err)
				}
			}
			continue
		}
		remotePath := rel
		if remotePrefix != "" {
			remotePath = remotePrefix + "/" + rel
		}
		syncLog("↑ %s", remotePath)
		if _, err := c.Upload(localPath, remotePath); err != nil {
			return fmt.Errorf("upload %s: %w", remotePath, err)
		}
		_ = writeHydratedMeta(localRoot, rel, placeholderMeta{Hash: local[rel], Size: re.Size})
		_ = ClearLocalModify(localRoot, rel)
		if oldRemote, ok := legacyRemotes[rel]; ok && oldRemote != remotePath {
			syncLog("↺ remove legacy %s", oldRemote)
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
			syncLog("☁ placeholder %s", rel)
			if err := writePlaceholder(localRoot, rel, placeholderMeta{Hash: e.Hash, Size: e.Size}); err != nil {
				fmt.Fprintf(os.Stderr, "aviso: placeholder %s: %v\n", rel, err)
				continue
			}
			continue
		}
		syncLog("↓ %s", downloadPath)
		if err := c.Download(downloadPath, localPath); err != nil {
			return fmt.Errorf("download %s: %w", downloadPath, err)
		}
		if legacyRemote, ok := legacyRemotes[rel]; ok {
			syncLog("↺ remove legacy %s", legacyRemote)
			_ = c.Delete(legacyRemote)
		}
	}

	// Download pinned paths that are still placeholders.
	if err := hydratePinnedFromManifest(c, localRoot, &st, remote, legacyRemotes); err != nil {
		return err
	}

	removeEmptyLegacyDirs(localRoot)

	syncLog("sync: reindexando pasta local...")
	local, err = scanLocalFilesForSync(localRoot, filterKnownExcludingDeletes(localRoot, st.Known))
	if err != nil {
		return err
	}
	localDirs, err = scanLocalDirsForSync(localRoot)
	if err != nil {
		return err
	}
	st.Known = local
	st.KnownDirs = localDirs
	st.Entries = rebuildEntries(localRoot, local, remote, st)

	newCursor, err = applyRemoteChanges(c, localRoot, st.ChangeCursor)
	if err != nil {
		if IsConnectionError(err) {
			fmt.Fprintf(os.Stderr, "aviso: feed de changes (pos-sync) indisponivel: %v\n", err)
		} else {
			return fmt.Errorf("apply changes after sync: %w", err)
		}
	} else {
		st.ChangeCursor = newCursor
	}

	st.LastManifestFP = fp
	return SaveStateCached(statePath, st)
}

func hydratePinnedFromManifest(c *Client, localRoot string, st *SyncState, remote map[string]ManifestEntry, legacyRemotes map[string]string) error {
	for rel, e := range remote {
		if !isPinnedPath(st.Pinned, rel) {
			continue
		}
		localPath := placeholderPath(localRoot, rel)
		if cfapiProviderActive() {
			if !IsPlaceholderRel(localRoot, rel) {
				continue
			}
			if err := providerHydrate(localRoot, rel); err != nil {
				return fmt.Errorf("hydrate pinned %s: %w", rel, err)
			}
			continue
		}
		if !IsPlaceholderRel(localRoot, rel) {
			if h, _, err := FileHash(localPath); err == nil && h == e.Hash {
				continue
			}
		}
		downloadPath := e.Path
		if legacyRemote, ok := legacyRemotes[rel]; ok {
			downloadPath = legacyRemote
		}
		syncLog("↓ pinned %s", rel)
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
		} else if IsPlaceholderRel(localRoot, rel) {
			entry.Availability = AvPlaceholder
		}
		if re, ok := remote[rel]; ok {
			entry.Size = re.Size
		}
		entries[rel] = entry
	}
	return entries
}
