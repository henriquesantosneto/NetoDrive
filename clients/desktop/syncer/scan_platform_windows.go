//go:build windows

package syncer

import "time"

func scanLocalFilesForSync(localRoot string, known map[string]string) (map[string]string, error) {
	if providerExe() != "" {
		return scanLocalFilesLightWithTimeout(localRoot, known, 30*time.Second)
	}
	return scanLocalFilesWithTimeout(localRoot, 2*time.Minute)
}

func scanLocalDirsForSync(localRoot string) (map[string]bool, error) {
	if providerExe() != "" {
		return scanLocalDirsWithTimeout(localRoot, 15*time.Second)
	}
	return scanLocalDirsLight(localRoot)
}
