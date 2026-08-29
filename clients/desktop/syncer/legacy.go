package syncer

import (
	"os"
	"path/filepath"
	"strings"
)

// legacyDevicePrefixes were remote_prefix values before the unified account tree.
var legacyDevicePrefixes = []string{"PC", "Android"}

// localRelFromRemote strips a legacy device prefix so PC/foo.txt syncs as foo.txt locally.
func localRelFromRemote(remotePath string) (rel string, legacyRemote string) {
	remotePath = strings.Trim(remotePath, "/")
	for _, prefix := range legacyDevicePrefixes {
		if remotePath == prefix {
			return "", remotePath
		}
		p := prefix + "/"
		if strings.HasPrefix(remotePath, p) {
			return strings.TrimPrefix(remotePath, p), remotePath
		}
	}
	return remotePath, ""
}

// localRelFromLocal strips a legacy folder under the sync root on upload.
func localRelFromLocal(rel string) string {
	rel = strings.Trim(rel, "/")
	for _, prefix := range legacyDevicePrefixes {
		if rel == prefix {
			return ""
		}
		p := prefix + "/"
		if strings.HasPrefix(rel, p) {
			return strings.TrimPrefix(rel, p)
		}
	}
	return rel
}

func legacyRemotesFromManifest(man *Manifest, remotePrefix string) map[string]string {
	legacy := map[string]string{}
	if man == nil {
		return legacy
	}
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
		rel, legacyRemote := localRelFromRemote(path)
		if rel == "" || legacyRemote == "" {
			continue
		}
		legacy[rel] = e.Path
	}
	return legacy
}

func removeEmptyLegacyDirs(localRoot string) {
	if cfapiProviderActive() {
		return
	}
	for _, prefix := range legacyDevicePrefixes {
		dir := filepath.Join(localRoot, prefix)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			_ = os.Remove(dir)
		}
	}
}
