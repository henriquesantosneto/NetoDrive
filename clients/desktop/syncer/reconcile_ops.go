package syncer

import "strings"

func manifestHasPath(man *Manifest, remotePath string) bool {
	if man == nil {
		return false
	}
	remotePath = strings.Trim(strings.ReplaceAll(remotePath, "\\", "/"), "/")
	for _, e := range man.Files {
		if e.IsDir {
			continue
		}
		if strings.Trim(e.Path, "/") == remotePath {
			return true
		}
	}
	return false
}

func manifestPathsForRel(rel, remotePrefix string, legacyRemotes map[string]string) (oldRemotes []string, newRemote string) {
	rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
	newRemote = rel
	if remotePrefix != "" {
		newRemote = remotePrefix + "/" + rel
	}
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.Trim(p, "/")
		if p != "" && !seen[p] {
			seen[p] = true
			oldRemotes = append(oldRemotes, p)
		}
	}
	add(remoteDeletePath(rel, remotePrefix, legacyRemotes))
	if leg, ok := legacyRemotes[rel]; ok {
		add(leg)
	}
	add(rel)
	return oldRemotes, newRemote
}

type renameDecision int

const (
	renameDecisionApply renameDecision = iota
	renameDecisionDone
	renameDecisionCancelRemoteDelete
)

// decidePendingRename applies OneDrive-like rules against the current remote manifest.
func decidePendingRename(man *Manifest, from, to, remotePrefix string, legacyRemotes map[string]string) renameDecision {
	oldCandidates, newRemote := manifestPathsForRel(from, remotePrefix, legacyRemotes)
	_, newRemoteTo := manifestPathsForRel(to, remotePrefix, legacyRemotes)

	oldExists := false
	for _, p := range oldCandidates {
		if manifestHasPath(man, p) {
			oldExists = true
			break
		}
	}
	newExists := manifestHasPath(man, newRemoteTo) || manifestHasPath(man, newRemote)

	if newExists && !oldExists {
		return renameDecisionDone
	}
	if !oldExists && !newExists {
		return renameDecisionCancelRemoteDelete
	}
	return renameDecisionApply
}

func cancelPendingRename(localRoot string, st *SyncState, rn localRename, reason string) error {
	from, to := rn.From, rn.To
	syncLog("↪ rename cancelado (%s): %s -> %s", reason, from, to)
	_ = deleteLocalFile(localRoot, to)
	_ = deleteLocalFile(localRoot, from)
	removePlatformPlaceholder(localRoot, to)
	removePlatformPlaceholder(localRoot, from)
	delete(st.Known, from)
	delete(st.Known, to)
	delete(st.Entries, from)
	delete(st.Entries, to)
	_ = ClearLocalModify(localRoot, from)
	_ = ClearLocalModify(localRoot, to)
	return ClearLocalRename(localRoot, rn)
}

// CancelPendingRenamesForDeletedRemote cancels queued renames when the web/other client removed the source path.
func CancelPendingRenamesForDeletedRemote(localRoot string, st *SyncState, deletedRel string) error {
	deletedRel = strings.Trim(strings.ReplaceAll(deletedRel, "\\", "/"), "/")
	set, err := PendingLocalRenameSet(localRoot)
	if err != nil || len(set) == 0 {
		return err
	}
	for _, rn := range set {
		if rn.From != deletedRel {
			continue
		}
		if err := cancelPendingRename(localRoot, st, rn, "removido na web"); err != nil {
			return err
		}
	}
	return nil
}
