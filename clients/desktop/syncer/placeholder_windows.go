//go:build windows

package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	if vbs := defaultOpenPlaceholderVBS(); vbs != "" {
		openPlaceholderVBS = vbs
	}
}

var openPlaceholderVBS string

func defaultOpenPlaceholderVBS() string {
	localApp := os.Getenv("LOCALAPPDATA")
	if localApp == "" {
		return ""
	}
	p := filepath.Join(localApp, "NetoDrive", "OpenPlaceholder.vbs")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// SetOpenPlaceholderVBS sets the VBS launcher used for Windows shortcut placeholders.
func SetOpenPlaceholderVBS(path string) {
	openPlaceholderVBS = path
}

func placeholderDiskPath(localRoot, rel string) string {
	return placeholderPath(localRoot, rel) + ".lnk"
}

func providerExe() string {
	localApp := os.Getenv("LOCALAPPDATA")
	if localApp == "" {
		return ""
	}
	p := filepath.Join(localApp, "NetoDrive", "netodrive-provider.exe")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func cfapiProviderActive() bool {
	return providerExe() != ""
}

func CfapiProviderInstalled() bool { return cfapiProviderActive() }

func writePlatformPlaceholder(localRoot, rel string, meta placeholderMeta) error {
	meta.CloudOnly = boolPtr(true)
	if err := writePlaceholderMeta(localRoot, rel, meta); err != nil {
		return err
	}
	if placeholderUpToDate(localRoot, rel, meta) {
		return nil
	}
	// Never write into the CFAPI sync root or spawn a second provider process (deadlocks Explorer).
	if cfapiProviderActive() {
		return enqueuePlaceholder(localRoot, rel, meta)
	}
	if placeholderBulkSync {
		return writePlaceholderMagic(localRoot, rel, meta)
	}
	lnk := placeholderDiskPath(localRoot, rel)
	if err := os.MkdirAll(filepath.Dir(lnk), 0o755); err != nil {
		return err
	}
	_ = os.Remove(placeholderPath(localRoot, rel))

	vbs := openPlaceholderVBS
	if vbs == "" {
		vbs = defaultOpenPlaceholderVBS()
	}
	if vbs == "" {
		return writePlaceholderMagic(localRoot, rel, meta)
	}

	relSlash := filepath.ToSlash(rel)
	args := fmt.Sprintf("//nologo %q %q", vbs, relSlash)
	ps := fmt.Sprintf(
		`$s=(New-Object -ComObject WScript.Shell).CreateShortcut(%q);$s.TargetPath='wscript.exe';$s.Arguments=%q;$s.WorkingDirectory=%q;$s.WindowStyle=7;$s.Description='NetoDrive — nuvem';$s.Save()`,
		lnk,
		args,
		filepath.Dir(lnk),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create shortcut %s: %w (%s)", lnk, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func defaultConfigForProvider() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "NetoDrive", "netodrive.json")
}

func isCloudPlaceholder(path string) bool {
	if providerExe() == "" {
		return false
	}
	localRoot, rel, ok := findSyncRootForPath(path)
	if !ok {
		return false
	}
	if _, ok := readPlaceholderMetaForRel(localRoot, rel); !ok {
		return false
	}
	if !isCloudOnlyPlaceholder(localRoot, rel) {
		return false
	}
	// Sidecar present — cloud placeholder until fully hydrated (meta removed on hydrate).
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func findSyncRootForPath(path string) (localRoot, rel string, ok bool) {
	cfg := defaultConfigForProvider()
	if cfg == "" {
		return "", "", false
	}
	b, err := os.ReadFile(cfg)
	if err != nil {
		return "", "", false
	}
	var doc struct {
		LocalFolder string `json:"local_folder"`
	}
	if err := json.Unmarshal(b, &doc); err != nil || doc.LocalFolder == "" {
		return "", "", false
	}
	localRoot = ResolveLocalFolder(cfg, doc.LocalFolder)
	localRoot, err = filepath.Abs(localRoot)
	if err != nil {
		return "", "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", false
	}
	rel, err = filepath.Rel(localRoot, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", false
	}
	return localRoot, filepath.ToSlash(rel), true
}

func isPlatformPlaceholder(path string) bool {
	if isCloudPlaceholder(path) {
		return true
	}
	if strings.HasSuffix(strings.ToLower(path), ".lnk") {
		return isNetoDriveShortcut(path)
	}
	if IsPlaceholderMagicFile(path) {
		return true
	}
	// Content path: check companion .lnk
	if _, err := os.Stat(path + ".lnk"); err == nil {
		return isNetoDriveShortcut(path + ".lnk")
	}
	return false
}

func isNetoDriveShortcut(lnkPath string) bool {
	vbs := openPlaceholderVBS
	if vbs == "" {
		vbs = defaultOpenPlaceholderVBS()
	}
	if vbs == "" {
		return false
	}
	b, err := os.ReadFile(lnkPath)
	if err != nil || len(b) < 20 {
		return false
	}
	// LNK is UTF-16LE with embedded strings; look for our VBS name.
	lower := strings.ToLower(string(b))
	return strings.Contains(lower, strings.ToLower(filepath.Base(vbs))) ||
		strings.Contains(lower, "openplaceholder.vbs")
}

func removePlatformPlaceholder(localRoot, rel string) {
	_ = os.Remove(placeholderPath(localRoot, rel))
	_ = os.Remove(placeholderDiskPath(localRoot, rel))
	removePlaceholderMeta(localRoot, rel)
	_ = removePlaceholderQueueRel(localRoot, rel)
}

func deleteLocalFilePlatform(localRoot, rel string) error {
	if exe := providerExe(); exe != "" {
		cfg := defaultConfigForProvider()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, "-remove", filepath.ToSlash(rel), "-config", cfg)
		_ = cmd.Run()
	}
	return nil
}

func runProviderCommand(args ...string) error {
	exe := providerExe()
	if exe == "" {
		return fmt.Errorf("netodrive-provider nao instalado")
	}
	cfg := defaultConfigForProvider()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmdArgs := append(append([]string{}, args...), "-config", cfg)
	cmd := exec.CommandContext(ctx, exe, cmdArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func providerPin(localRoot, rel string) error {
	return runProviderOp(localRoot, "pin", rel)
}

func providerHydrate(localRoot, rel string) error {
	return runProviderOp(localRoot, "hydrate", rel)
}

func providerDehydrate(localRoot, rel string) error {
	return runProviderOp(localRoot, "dehydrate", rel)
}

func ensureProviderProcess(localRoot string) error {
	if providerRunProcessActive() {
		return nil
	}
	cfg := defaultConfigForProvider()
	if cfg == "" {
		return fmt.Errorf("netodrive.json nao encontrado")
	}
	return startProviderRunProcess(cfg)
}

func providerRunProcessActive() bool {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq netodrive-provider.exe", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "netodrive-provider.exe")
}

func startProviderRunProcess(cfgPath string) error {
	exe := providerExe()
	if exe == "" {
		return fmt.Errorf("netodrive-provider nao instalado")
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	logDir := filepath.Join(appData, "NetoDrive", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "provider.log")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	reg := exec.Command(exe, "-ensure-register", "-config", cfgPath)
	reg.Stdout = logFile
	reg.Stderr = logFile
	_ = reg.Run()

	cmd := exec.Command(exe, "-run", "-config", cfgPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("iniciar provider: %w", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if providerRunProcessActive() {
			time.Sleep(500 * time.Millisecond)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("provider CFAPI nao iniciou (veja %s)", logPath)
}

func ensureCFAPIPlaceholder(localRoot, rel string, meta placeholderMeta) error {
	if !cfapiProviderActive() {
		return nil
	}
	path := placeholderPath(localRoot, rel)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := writePlaceholder(localRoot, rel, meta); err != nil {
		return err
	}
	if err := ensureProviderProcess(localRoot); err != nil {
		return err
	}
	return waitForLocalPlaceholder(path, 45*time.Second)
}

func waitForLocalPlaceholder(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout aguardando placeholder local: %s", path)
}

// ResolveOpenRel maps a double-click path (maybe .lnk) to account-relative path.
func ResolveOpenRel(localRoot, argPath string) string {
	argPath = strings.TrimSpace(argPath)
	if argPath == "" {
		return ""
	}
	abs, err := filepath.Abs(argPath)
	if err != nil {
		abs = argPath
	}
	localRoot, _ = filepath.Abs(localRoot)

	if strings.HasSuffix(strings.ToLower(abs), ".lnk") {
		if rel, ok := relFromLnkUnderRoot(localRoot, abs); ok {
			return rel
		}
	}
	if rel, err := filepath.Rel(localRoot, abs); err == nil {
		rel = filepath.ToSlash(rel)
		if !strings.HasPrefix(rel, "..") {
			rel = strings.TrimSuffix(rel, ".lnk")
			return strings.Trim(rel, "/")
		}
	}
	return filepath.ToSlash(strings.Trim(strings.TrimSuffix(argPath, ".lnk"), "/"))
}

func relFromLnkUnderRoot(localRoot, lnkAbs string) (string, bool) {
	rel, err := filepath.Rel(localRoot, lnkAbs)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return strings.TrimSuffix(rel, ".lnk"), true
}
