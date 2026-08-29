package syncer

type syncPlan struct {
	upload       []string
	download     []string
	deleteLocal  []string
	deleteRemote []string
}

// planSync decides how to reconcile local disk with the remote manifest using
// the last known synced snapshot (paths the client previously mirrored).
func planSync(local, remote, known map[string]string) syncPlan {
	var p syncPlan

	for rel, hash := range local {
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
		if _, inLocal := local[rel]; inLocal {
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
