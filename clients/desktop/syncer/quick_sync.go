package syncer

// TryQuickSync returns true when the remote manifest matches the last successful sync.
// Safe under CFAPI: HTTP only, no local folder access.
func TryQuickSync(c *Client, statePath, localRoot string) (bool, error) {
	st, err := LoadStateCached(statePath, localRoot)
	if err != nil {
		return false, err
	}
	if st.LastManifestFP == "" {
		return false, nil
	}
	if err := c.Ping(); err != nil {
		return false, err
	}
	man, err := c.Manifest()
	if err != nil {
		return false, err
	}
	fp := manifestFingerprint(man)
	if HasPendingLocalChanges(localRoot) {
		return false, nil
	}
	localDirs, err := scanLocalDirsForSync(localRoot)
	if err == nil && dirsChanged(localDirs, st.KnownDirs) {
		return false, nil
	}
	if fp != "" && fp == st.LastManifestFP {
		return true, nil
	}
	return false, nil
}

// RemoteManifestChanged reports whether the server manifest differs from the last sync.
func RemoteManifestChanged(c *Client, statePath, localRoot string) (bool, error) {
	st, err := LoadStateCached(statePath, localRoot)
	if err != nil {
		return false, err
	}
	if st.LastManifestFP == "" {
		return true, nil
	}
	if err := c.Ping(); err != nil {
		return false, err
	}
	man, err := c.Manifest()
	if err != nil {
		return false, err
	}
	fp := manifestFingerprint(man)
	return fp != st.LastManifestFP, nil
}
