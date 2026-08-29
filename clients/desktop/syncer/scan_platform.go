//go:build !windows

package syncer

import "time"

func scanLocalFilesForSync(localRoot string, known map[string]string) (map[string]string, error) {
	return scanLocalFilesWithTimeout(localRoot, 2*time.Minute)
}
