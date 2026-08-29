package syncer

import (
	"os"
	"path/filepath"
)

// IsLikelyRepoRoot reports whether dir looks like a NetoDrive/git checkout (not user sync data).
func IsLikelyRepoRoot(dir string) bool {
	for _, marker := range []string{".git", filepath.Join("server", "go.mod"), filepath.Join("clients", "desktop")} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}
