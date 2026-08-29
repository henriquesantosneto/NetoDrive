package syncer

import "fmt"

// applyPendingLocalRenames reconciles Explorer renames against the remote manifest (OneDrive-like).
func applyPendingLocalRenames(c *Client, localRoot, remotePrefix string, legacyRemotes map[string]string, man *Manifest, st *SyncState) error {
	set, err := PendingLocalRenameSet(localRoot)
	if err != nil || len(set) == 0 {
		return err
	}
	for _, rn := range set {
		switch decidePendingRename(man, rn.From, rn.To, remotePrefix, legacyRemotes) {
		case renameDecisionCancelRemoteDelete:
			if err := cancelPendingRename(localRoot, st, rn, "arquivo removido na web"); err != nil {
				return err
			}
			continue
		case renameDecisionDone:
			renameLocalState(localRoot, st, rn.From, rn.To)
			if err := ClearLocalRename(localRoot, rn); err != nil {
				return err
			}
			continue
		}

		if err := renameRemotePaths(c, man, rn.From, rn.To, remotePrefix, legacyRemotes); err != nil {
			if decidePendingRename(man, rn.From, rn.To, remotePrefix, legacyRemotes) == renameDecisionCancelRemoteDelete {
				if err2 := cancelPendingRename(localRoot, st, rn, "arquivo removido na web"); err2 != nil {
					return err2
				}
				continue
			}
			return fmt.Errorf("rename remote %s -> %s: %w", rn.From, rn.To, err)
		}
		renameLocalState(localRoot, st, rn.From, rn.To)
		if err := ClearLocalRename(localRoot, rn); err != nil {
			return err
		}
	}
	return nil
}
