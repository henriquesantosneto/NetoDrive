package syncer

import (
	"fmt"
)

// applyPendingLocalDeletes removes server copies for Explorer deletes queued by the CFAPI provider.
func applyPendingLocalDeletes(c *Client, localRoot, remotePrefix string, legacyRemotes map[string]string, st *SyncState) error {
	pending, err := PendingLocalDeleteSet(localRoot)
	if err != nil || len(pending) == 0 {
		return nil
	}
	for rel := range pending {
		remotePath := remoteDeletePath(rel, remotePrefix, legacyRemotes)
		syncLog("× remoto %s (delete local pendente)", remotePath)
		if err := c.Delete(remotePath); err != nil {
			return fmt.Errorf("delete remote %s: %w", remotePath, err)
		}
		if remotePath != rel {
			_ = c.Delete(rel)
		}
		removePlatformPlaceholder(localRoot, rel)
		delete(st.Known, rel)
		delete(st.Entries, rel)
		if err := ClearLocalDelete(localRoot, rel); err != nil {
			return err
		}
	}
	return nil
}
