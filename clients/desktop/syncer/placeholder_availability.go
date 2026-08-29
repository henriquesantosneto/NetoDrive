package syncer

import "os"

func boolPtr(v bool) *bool {
	return &v
}

func metaCloudOnly(m placeholderMeta) bool {
	if m.CloudOnly == nil {
		return true
	}
	return *m.CloudOnly
}

// isCloudOnlyPlaceholder reports dehydrated/cloud-only files (not fully local).
func isCloudOnlyPlaceholder(localRoot, rel string) bool {
	if meta, ok := readPlaceholderMetaForRel(localRoot, rel); ok {
		return metaCloudOnly(meta)
	}
	path := placeholderPath(localRoot, rel)
	if IsPlaceholderMagicFile(path) {
		return true
	}
	if cfapiProviderActive() {
		return false
	}
	return isPlatformPlaceholder(path)
}

func localHashForSync(localRoot, rel string) (string, error) {
	if isCloudOnlyPlaceholder(localRoot, rel) {
		if meta, ok := readPlaceholderMetaForRel(localRoot, rel); ok {
			return meta.Hash, nil
		}
	}
	h, _, err := FileHash(placeholderPath(localRoot, rel))
	return h, err
}

func writeCloudOnlyMeta(localRoot, rel string, meta placeholderMeta) error {
	meta.CloudOnly = boolPtr(true)
	return writePlaceholderMeta(localRoot, rel, meta)
}

func writeHydratedMeta(localRoot, rel string, meta placeholderMeta) error {
	meta.CloudOnly = boolPtr(false)
	return writePlaceholderMeta(localRoot, rel, meta)
}

func indexLocalFileHash(localRoot, rel string) (string, bool) {
	path := placeholderPath(localRoot, rel)
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return "", false
	}
	h, err := localHashForSync(localRoot, rel)
	if err != nil {
		return "", false
	}
	return h, true
}

func localContentChangedSinceSync(localRoot string, known map[string]string) bool {
	if !cfapiProviderActive() || len(known) == 0 {
		return false
	}
	for rel, syncedHash := range known {
		if isCloudOnlyPlaceholder(localRoot, rel) {
			continue
		}
		h, err := localHashForSync(localRoot, rel)
		if err != nil {
			continue
		}
		if h != syncedHash {
			return true
		}
	}
	return false
}
