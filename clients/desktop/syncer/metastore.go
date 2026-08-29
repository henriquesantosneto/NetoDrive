package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

func metaKeyFromRel(rel string) string {
	key := strings.ReplaceAll(filepath.ToSlash(rel), "/", "__")
	if key == "" {
		return "_root"
	}
	return key
}

func metaRelFromKey(key string) string {
	if key == "_root" {
		return ""
	}
	return strings.ReplaceAll(key, "__", "/")
}

// syncRootDataID is a stable id for per-sync-root data under %APPDATA%/NetoDrive/.
func syncRootDataID(localRoot string) string {
	abs, err := filepath.Abs(localRoot)
	if err != nil {
		abs = localRoot
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(abs))))
	return hex.EncodeToString(sum[:8])
}

// Meta lives outside the CFAPI sync root (mkdir .netodrive inside sync root fails on Windows).
func metaStoreRoot(localRoot string) string {
	id := syncRootDataID(localRoot)
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "NetoDrive", "placeholder-meta", id)
}

func metaSidecarPath(localRoot, rel string) string {
	return filepath.Join(metaStoreRoot(localRoot), metaKeyFromRel(rel)+".json")
}

func legacyMetaSidecarPath(localRoot, rel string) string {
	return filepath.Join(localRoot, ".netodrive", "meta", metaKeyFromRel(rel)+".json")
}

func migrateLegacyMetaSidecar(localRoot, rel string) {
	newPath := metaSidecarPath(localRoot, rel)
	if _, err := os.Stat(newPath); err == nil {
		return
	}
	if cfapiProviderActive() {
		return
	}
	oldPath := legacyMetaSidecarPath(localRoot, rel)
	b, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(newPath), 0o755)
	_ = os.WriteFile(newPath, b, 0o644)
	_ = os.Remove(oldPath)
}
