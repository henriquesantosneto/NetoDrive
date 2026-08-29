package syncer

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type localPinOp struct {
	Op  string `json:"op"`
	Rel string `json:"rel"`
}

func pendingPinOpsPath(localRoot string) string {
	return filepath.Join(localChangesRoot(localRoot), "pending-pin-ops.jsonl")
}

func EnqueueLocalPinOp(localRoot, op, rel string) error {
	op = strings.TrimSpace(strings.ToLower(op))
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	if rel == "" || (op != "pin" && op != "unpin") {
		return nil
	}
	path := pendingPinOpsPath(localRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(localPinOp{Op: op, Rel: rel})
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func applyPendingPinOps(statePath, localRoot string) error {
	path := pendingPinOpsPath(localRoot)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var ops []localPinOp
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var op localPinOp
		if err := json.Unmarshal([]byte(line), &op); err != nil {
			continue
		}
		op.Rel = filepath.ToSlash(strings.Trim(op.Rel, "/"))
		if op.Rel == "" {
			continue
		}
		ops = append(ops, op)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(ops) == 0 {
		return nil
	}

	for _, op := range ops {
		switch op.Op {
		case "pin":
			if err := PinPath(statePath, op.Rel); err != nil {
				return err
			}
			if meta, ok := readPlaceholderMetaForRel(localRoot, op.Rel); ok {
				_ = writeHydratedMeta(localRoot, op.Rel, meta)
			}
		case "unpin":
			if err := UnpinPath(statePath, op.Rel); err != nil {
				return err
			}
			if meta, ok := readPlaceholderMetaForRel(localRoot, op.Rel); ok {
				_ = writeCloudOnlyMeta(localRoot, op.Rel, meta)
			}
		}
	}
	return os.Remove(path)
}
