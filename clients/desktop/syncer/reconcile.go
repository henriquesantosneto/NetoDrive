package syncer

type syncPlan struct {
	upload       []string
	download     []string
	deleteLocal  []string
	deleteRemote []string
}

// PlanSyncOptions tunes reconciliation for CFAPI placeholder sync roots.
type PlanSyncOptions struct {
	LocalRoot            string
	RematerializeMissing bool
	PendingLocalDeletes  map[string]bool
	PendingLocalRenames  map[string]string // from -> to
}

// planSync decides how to reconcile local disk with the remote manifest using
// the last known synced snapshot (paths the client previously mirrored).
func planSync(local, remote, known map[string]string, opts PlanSyncOptions) syncPlan {
	var p syncPlan

	for rel, hash := range local {
		if opts.PendingLocalRenames != nil {
			skip := false
			for from, to := range opts.PendingLocalRenames {
				if rel != to {
					continue
				}
				if remoteHash, ok := remote[from]; ok && remoteHash == hash {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		remoteHash, inRemote := remote[rel]
		_, wasKnown := known[rel]
		if inRemote {
			if remoteHash == hash {
				continue
			}
			p.upload = append(p.upload, rel)
			continue
		}
		if wasKnown {
			p.deleteLocal = append(p.deleteLocal, rel)
			continue
		}
		p.upload = append(p.upload, rel)
	}

	for rel := range remote {
		if opts.PendingLocalRenames != nil {
			if _, pending := opts.PendingLocalRenames[rel]; pending {
				continue
			}
		}
		if _, inLocal := local[rel]; inLocal {
			continue
		}
		if opts.PendingLocalDeletes != nil && opts.PendingLocalDeletes[rel] {
			p.deleteRemote = append(p.deleteRemote, rel)
			continue
		}
		if opts.RematerializeMissing && shouldRematerializePlaceholder(opts.LocalRoot, rel, opts.PendingLocalDeletes) {
			p.download = append(p.download, rel)
			continue
		}
		if _, wasKnown := known[rel]; wasKnown {
			p.deleteRemote = append(p.deleteRemote, rel)
			continue
		}
		p.download = append(p.download, rel)
	}

	return p
}
