//go:build !windows

package syncer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func placeholderDiskPath(localRoot, rel string) string {
	return placeholderPath(localRoot, rel)
}

func writePlatformPlaceholder(localRoot, rel string, meta placeholderMeta) error {
	return writePlaceholderMagic(localRoot, rel, meta)
}

func isPlatformPlaceholder(path string) bool {
	return IsPlaceholderMagicFile(path)
}

func removePlatformPlaceholder(localRoot, rel string) {
	_ = os.Remove(placeholderPath(localRoot, rel))
	removePlaceholderMeta(localRoot, rel)
	_ = removePlaceholderQueueRel(localRoot, rel)
}

func deleteLocalFilePlatform(localRoot, rel string) error {
	return nil
}

func providerPin(localRoot, rel string) error { return nil }

func providerHydrate(localRoot, rel string) error { return nil }

func providerDehydrate(localRoot, rel string) error { return nil }

func ensureProviderProcess(localRoot string) error { return nil }

func ensureCFAPIPlaceholder(localRoot, rel string, meta placeholderMeta) error { return nil }

func waitForLocalPlaceholder(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout aguardando arquivo local: %s", path)
}

// ResolveOpenRel maps a path under localRoot to account-relative path.
func ResolveOpenRel(localRoot, argPath string) string {
	abs, err := filepath.Abs(argPath)
	if err != nil {
		abs = argPath
	}
	localRoot, _ = filepath.Abs(localRoot)
	rel, err := filepath.Rel(localRoot, abs)
	if err != nil {
		return strings.Trim(filepath.ToSlash(argPath), "/")
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "..") {
		return strings.Trim(filepath.ToSlash(argPath), "/")
	}
	return strings.Trim(rel, "/")
}
