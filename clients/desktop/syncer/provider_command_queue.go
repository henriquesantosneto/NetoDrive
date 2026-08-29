package syncer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type providerCommandEntry struct {
	Op  string `json:"op"`
	Rel string `json:"rel"`
}

func providerCommandsRoot(localRoot string) string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "NetoDrive", "provider-commands", syncRootDataID(localRoot))
}

func providerCommandsPath(localRoot string) string {
	return filepath.Join(providerCommandsRoot(localRoot), "pending.jsonl")
}

func enqueueProviderCommand(localRoot, op, rel string) error {
	op = strings.ToLower(strings.TrimSpace(op))
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	if op == "" || rel == "" {
		return nil
	}
	if isProviderCommandQueued(localRoot, op, rel) {
		return nil
	}
	path := providerCommandsPath(localRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	entry := providerCommandEntry{Op: op, Rel: rel}
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func isProviderCommandQueued(localRoot, op, rel string) bool {
	op = strings.ToLower(strings.TrimSpace(op))
	rel = filepath.ToSlash(strings.Trim(rel, "/"))
	path := providerCommandsPath(localRoot)
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var entry providerCommandEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if strings.EqualFold(entry.Op, op) && filepath.ToSlash(strings.Trim(entry.Rel, "/")) == rel {
			return true
		}
	}
	return false
}

func runProviderOp(localRoot, op, rel string) error {
	if providerExe() == "" {
		return fmt.Errorf("netodrive-provider nao instalado")
	}
	return enqueueProviderCommand(localRoot, op, rel)
}
