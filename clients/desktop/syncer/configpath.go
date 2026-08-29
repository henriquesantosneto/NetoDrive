package syncer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultSyncFolder returns the recommended local sync directory for this OS.
func DefaultSyncFolder() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "NetoDrive")
	}
	return filepath.Join(home, "Documents", "NetoDrive")
}

// ResolveLocalFolder turns config local_folder into an absolute path.
// Relative paths are resolved against the config file directory, not the process CWD.
func ResolveLocalFolder(cfgPath, folder string) string {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return DefaultSyncFolder()
	}
	if folder == "~" || strings.HasPrefix(folder, "~/") || strings.HasPrefix(folder, "~\\") {
		home, _ := os.UserHomeDir()
		if folder == "~" {
			return home
		}
		folder = filepath.Join(home, folder[2:])
	}
	if filepath.IsAbs(folder) {
		abs, _ := filepath.Abs(folder)
		return abs
	}
	base := filepath.Dir(cfgPath)
	return filepath.Clean(filepath.Join(base, folder))
}

// IsUnderOneDrive reports whether path lives inside a OneDrive sync directory.
func IsUnderOneDrive(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, env := range []string{"OneDrive", "OneDriveCommercial", "OneDriveConsumer"} {
		base := os.Getenv(env)
		if base == "" {
			continue
		}
		baseAbs, err := filepath.Abs(base)
		if err != nil {
			continue
		}
		if strings.EqualFold(abs, baseAbs) {
			return true
		}
		sep := string(os.PathSeparator)
		if strings.HasPrefix(strings.ToLower(abs), strings.ToLower(baseAbs+sep)) {
			return true
		}
	}
	return false
}
