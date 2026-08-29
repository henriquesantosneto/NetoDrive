package syncer

import "os"

var placeholderBulkSync bool

// SetPlaceholderBulkSync enables lightweight placeholder creation (no CFAPI subprocess per file).
func SetPlaceholderBulkSync(v bool) {
	placeholderBulkSync = v
}

func placeholderUpToDate(localRoot, rel string, meta placeholderMeta) bool {
	stored, ok := readPlaceholderMetaForRel(localRoot, rel)
	if !ok || stored.Hash != meta.Hash {
		return false
	}
	if isPlaceholderQueued(localRoot, rel, meta.Hash) {
		return true
	}
	if cfapiProviderActive() {
		return false
	}
	content := placeholderPath(localRoot, rel)
	if _, err := os.Stat(content); err == nil {
		if IsPlaceholderMagicFile(content) || isPlatformPlaceholder(content) {
			return true
		}
	}
	if _, err := os.Stat(placeholderDiskPath(localRoot, rel)); err == nil {
		return true
	}
	return false
}
