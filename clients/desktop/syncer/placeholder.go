package syncer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const placeholderMagic = "NETODRIVE_PLACEHOLDER_v1\n"

type placeholderMeta struct {
	Hash      string `json:"hash"`
	Size      int64  `json:"size"`
	CloudOnly *bool  `json:"cloud_only,omitempty"`
}

func placeholderPath(localRoot, rel string) string {
	return filepath.Join(localRoot, filepath.FromSlash(rel))
}

// IsPlaceholderMagicFile reports legacy magic-byte placeholder files.
func IsPlaceholderMagicFile(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil || len(b) < len(placeholderMagic) {
		return false
	}
	return string(b[:len(placeholderMagic)]) == placeholderMagic
}

func IsPlaceholderFile(path string) bool {
	return isPlatformPlaceholder(path)
}

// IsPlaceholderRel reports cloud-only placeholders (dehydrated), not hydrated local copies.
func IsPlaceholderRel(localRoot, rel string) bool {
	return isCloudOnlyPlaceholder(localRoot, rel)
}

func readPlaceholderMeta(path string) (placeholderMeta, bool) {
	if meta, ok := readPlaceholderMetaFile(path); ok {
		return meta, true
	}
	return placeholderMeta{}, false
}

func readPlaceholderMetaFile(path string) (placeholderMeta, bool) {
	if IsPlaceholderMagicFile(path) {
		b, err := os.ReadFile(path)
		if err != nil {
			return placeholderMeta{}, false
		}
		var meta placeholderMeta
		if err := json.Unmarshal(b[len(placeholderMagic):], &meta); err != nil {
			return placeholderMeta{}, false
		}
		return meta, true
	}
	return placeholderMeta{}, false
}

func readPlaceholderMetaForPath(localRoot, path, rel string) (placeholderMeta, bool) {
	if meta, ok := readPlaceholderMetaForRel(localRoot, rel); ok {
		return meta, true
	}
	if meta, ok := readPlaceholderMetaFile(path); ok {
		return meta, true
	}
	if _, err := os.Stat(placeholderDiskPath(localRoot, rel)); err == nil {
		if meta, ok := readPlaceholderMetaForRel(localRoot, rel); ok {
			return meta, true
		}
	}
	return placeholderMeta{}, false
}

func writePlaceholderMagic(localRoot, rel string, meta placeholderMeta) error {
	path := placeholderPath(localRoot, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	content := append([]byte(placeholderMagic), body...)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return err
	}
	return writePlaceholderMeta(localRoot, rel, meta)
}

func writePlaceholder(localRoot, rel string, meta placeholderMeta) error {
	return writePlatformPlaceholder(localRoot, rel, meta)
}

func hashForLocalPath(localRoot, rel string) (hash string, isPlaceholder bool, err error) {
	content := placeholderPath(localRoot, rel)
	if meta, ok := readPlaceholderMetaForRel(localRoot, rel); ok {
		if _, err := os.Stat(placeholderDiskPath(localRoot, rel)); err == nil {
			return meta.Hash, true, nil
		}
		if IsPlaceholderMagicFile(content) {
			return meta.Hash, true, nil
		}
	}
	if meta, ok := readPlaceholderMetaFile(content); ok {
		return meta.Hash, true, nil
	}
	hash, _, err = FileHash(content)
	return hash, false, err
}

func isPinnedPath(pinned []string, rel string) bool {
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	for _, p := range pinned {
		p = filepath.ToSlash(strings.Trim(p, "/"))
		if p == "" {
			continue
		}
		if rel == p {
			return true
		}
		if strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

func PinPath(statePath, target string) error {
	st, err := LoadStateCached(statePath, "")
	if err != nil {
		return err
	}
	target = filepath.ToSlash(strings.Trim(target, "/"))
	if target == "" {
		return nil
	}
	for _, p := range st.Pinned {
		if filepath.ToSlash(strings.Trim(p, "/")) == target {
			return SaveStateCached(statePath, st)
		}
	}
	st.Pinned = append(st.Pinned, target)
	return SaveStateCached(statePath, st)
}

func UnpinPath(statePath, target string) error {
	st, err := LoadStateCached(statePath, "")
	if err != nil {
		return err
	}
	target = filepath.ToSlash(strings.Trim(target, "/"))
	var next []string
	for _, p := range st.Pinned {
		if filepath.ToSlash(strings.Trim(p, "/")) != target {
			next = append(next, p)
		}
	}
	st.Pinned = next
	return SaveStateCached(statePath, st)
}

func PinLocalPath(c *Client, localRoot, statePath, target string, onDemand bool) error {
	if err := PinPath(statePath, target); err != nil {
		return err
	}
	target = filepath.ToSlash(strings.Trim(target, "/"))
	if target == "" {
		return nil
	}
	st, err := LoadStateCached(statePath, localRoot)
	if err != nil {
		return err
	}
	man, err := c.Manifest()
	if err != nil {
		return err
	}
	matched := 0
	var firstErr error
	for _, e := range man.Files {
		if e.IsDir {
			continue
		}
		rel, _ := localRelFromRemote(e.Path)
		if rel != target && !strings.HasPrefix(rel, target+"/") {
			continue
		}
		matched++
		if cfapiProviderActive() {
			meta := placeholderMeta{Hash: e.Hash, Size: e.Size}
			if err := ensureCFAPIPlaceholder(localRoot, rel, meta); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("placeholder %s: %w", rel, err)
				}
				continue
			}
			if err := providerPin(localRoot, rel); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("fixar %s: %w", rel, err)
				}
				continue
			}
			if st.Entries == nil {
				st.Entries = map[string]FileEntry{}
			}
			st.Entries[rel] = FileEntry{Hash: e.Hash, Size: e.Size, Availability: AvPinned}
			st.Known[rel] = e.Hash
			_ = writeHydratedMeta(localRoot, rel, meta)
			continue
		}
		if err := HydratePath(c, localRoot, statePath, rel); err != nil {
			return err
		}
	}
	if matched == 0 {
		return fmt.Errorf("nenhum arquivo remoto encontrado para: %s", target)
	}
	if cfapiProviderActive() {
		if err := SaveStateCached(statePath, st); err != nil {
			return err
		}
		return firstErr
	}
	return SyncFolder(c, localRoot, statePath, onDemand)
}

func UnpinLocalPath(c *Client, localRoot, statePath, target string, onDemand bool) error {
	if err := UnpinPath(statePath, target); err != nil {
		return err
	}
	if !onDemand {
		return nil
	}
	target = filepath.ToSlash(strings.Trim(target, "/"))
	st, err := LoadStateCached(statePath, localRoot)
	if err != nil {
		return err
	}
	man, err := c.Manifest()
	if err != nil {
		return err
	}
	matched := 0
	var firstErr error
	for _, e := range man.Files {
		if e.IsDir {
			continue
		}
		rel, _ := localRelFromRemote(e.Path)
		if rel != target && !strings.HasPrefix(rel, target+"/") {
			continue
		}
		if isPinnedPath(st.Pinned, rel) {
			continue
		}
		matched++
		if cfapiProviderActive() {
			meta := placeholderMeta{Hash: e.Hash, Size: e.Size}
			if err := ensureCFAPIPlaceholder(localRoot, rel, meta); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("placeholder %s: %w", rel, err)
				}
				continue
			}
			if err := providerDehydrate(localRoot, rel); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("liberar espaco %s: %w", rel, err)
				}
				continue
			}
			if err := writeCloudOnlyMeta(localRoot, rel, meta); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			fmt.Printf("☁ liberado %s\n", rel)
			if st.Entries == nil {
				st.Entries = map[string]FileEntry{}
			}
			st.Entries[rel] = FileEntry{Hash: e.Hash, Size: e.Size, Availability: AvPlaceholder}
			st.Known[rel] = e.Hash
			continue
		}
		entry := st.Entries[rel]
		hash, size := e.Hash, e.Size
		if entry.Hash != "" {
			hash = entry.Hash
		}
		if entry.Size > 0 {
			size = entry.Size
		}
		meta := placeholderMeta{Hash: hash, Size: size}
		if err := releaseToPlaceholder(localRoot, rel, meta); err != nil {
			return err
		}
	}
	if matched == 0 {
		return fmt.Errorf("nenhum arquivo remoto encontrado para: %s", target)
	}
	if cfapiProviderActive() {
		if err := SaveStateCached(statePath, st); err != nil {
			return err
		}
		return firstErr
	}
	return SyncFolder(c, localRoot, statePath, onDemand)
}

func loadStateFile(statePath string) (SyncState, error) {
	return LoadState(statePath, "")
}

func HydratePath(c *Client, localRoot, statePath, rel string) error {
	localRoot, err := filepath.Abs(localRoot)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	if rel == "" {
		return nil
	}

	st, err := LoadState(statePath, localRoot)
	if err != nil {
		return err
	}

	man, err := c.Manifest()
	if err != nil {
		return err
	}
	var remotePath string
	found := false
	for _, e := range man.Files {
		if e.IsDir {
			continue
		}
		nrel, legacy := localRelFromRemote(e.Path)
		if nrel != rel {
			continue
		}
		found = true
		remotePath = e.Path
		if legacy != "" {
			remotePath = legacy
		}
		break
	}
	if !found {
		return fmt.Errorf("remote file not found: %s", rel)
	}

	localPath := placeholderPath(localRoot, rel)
	if cfapiProviderActive() {
		meta := placeholderMeta{Hash: "", Size: 0}
		for _, e := range man.Files {
			nrel, _ := localRelFromRemote(e.Path)
			if nrel == rel {
				meta.Hash = e.Hash
				meta.Size = e.Size
				break
			}
		}
		if meta.Hash != "" {
			if err := ensureCFAPIPlaceholder(localRoot, rel, meta); err != nil {
				return err
			}
		}
		if err := providerHydrate(localRoot, rel); err != nil {
			return err
		}
		hash := ""
		if meta, ok := readPlaceholderMetaForRel(localRoot, rel); ok {
			hash = meta.Hash
		} else {
			for _, e := range man.Files {
				nrel, _ := localRelFromRemote(e.Path)
				if nrel == rel {
					hash = e.Hash
					break
				}
			}
		}
		if hash == "" {
			return fmt.Errorf("cannot resolve hash for %s", rel)
		}
		if st.Entries == nil {
			st.Entries = map[string]FileEntry{}
		}
		avail := AvHydrated
		if isPinnedPath(st.Pinned, rel) {
			avail = AvPinned
		}
		st.Entries[rel] = FileEntry{Hash: hash, Availability: avail}
		st.Known[rel] = hash
		size := int64(0)
		for _, e := range man.Files {
			nrel, _ := localRelFromRemote(e.Path)
			if nrel == rel {
				size = e.Size
				break
			}
		}
		_ = writeHydratedMeta(localRoot, rel, placeholderMeta{Hash: hash, Size: size})
		return SaveStateCached(statePath, st)
	}

	removePlatformPlaceholder(localRoot, rel)
	fmt.Printf("↓ hydrate %s\n", rel)
	if err := c.Download(remotePath, localPath); err != nil {
		return err
	}
	hash, _, err := FileHash(localPath)
	if err != nil {
		return err
	}
	if st.Entries == nil {
		st.Entries = map[string]FileEntry{}
	}
	avail := AvHydrated
	if isPinnedPath(st.Pinned, rel) {
		avail = AvPinned
	}
	st.Entries[rel] = FileEntry{Hash: hash, Availability: avail}
	st.Known[rel] = hash
	return SaveStateCached(statePath, st)
}

func HydratePinned(c *Client, localRoot, statePath string) error {
	st, err := LoadState(statePath, localRoot)
	if err != nil {
		return err
	}
	for rel, entry := range st.Entries {
		if entry.Availability != AvPlaceholder {
			continue
		}
		if !isPinnedPath(st.Pinned, rel) {
			continue
		}
		if err := HydratePath(c, localRoot, statePath, rel); err != nil {
			return err
		}
	}
	return nil
}

func LocalRelFromRemote(remotePath string) (rel string, legacyRemote string) {
	return localRelFromRemote(remotePath)
}

func HydrateTree(c *Client, localRoot, statePath, target string) error {
	target = filepath.ToSlash(strings.Trim(target, "/"))
	if target == "" {
		return fmt.Errorf("empty path")
	}
	man, err := c.Manifest()
	if err != nil {
		return err
	}
	var firstErr error
	matched := 0
	for _, e := range man.Files {
		if e.IsDir {
			continue
		}
		rel, _ := localRelFromRemote(e.Path)
		if rel != target && !strings.HasPrefix(rel, target+"/") {
			continue
		}
		matched++
		if err := HydratePath(c, localRoot, statePath, rel); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if matched == 0 {
		return fmt.Errorf("remote path not found: %s", target)
	}
	return firstErr
}

func releaseToPlaceholder(localRoot, rel string, meta placeholderMeta) error {
	return writePlaceholder(localRoot, rel, meta)
}

// migrateLegacyPlaceholders rewrites magic-byte placeholders as platform shortcuts (.lnk on Windows).
func migrateLegacyPlaceholders(localRoot string, remote map[string]ManifestEntry, onDemand bool, pinned []string) error {
	if !onDemand {
		return nil
	}
	if cfapiProviderActive() {
		return nil
	}
	for rel, e := range remote {
		if isPinnedPath(pinned, rel) {
			continue
		}
		content := placeholderPath(localRoot, rel)
		if !IsPlaceholderMagicFile(content) {
			continue
		}
		meta := placeholderMeta{Hash: e.Hash, Size: e.Size}
		if err := writePlaceholder(localRoot, rel, meta); err != nil {
			return err
		}
	}
	return nil
}
