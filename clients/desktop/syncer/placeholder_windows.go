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

func writePlatformPlaceholder(localRoot, rel string, meta placeholderMeta) error {
	if err := writePlaceholderMeta(localRoot, rel, meta); err != nil {
		return err
	}
	if placeholderUpToDate(localRoot, rel, meta) {
		return nil
	}
	// Bulk sync: meta + magic file only. Spawning CFAPI/PowerShell per file freezes Explorer.
	if placeholderBulkSync {
		return writePlaceholderMagic(localRoot, rel, meta)
	}
	if exe := providerExe(); exe != "" {
		cfg := defaultConfigForProvider()
		args := []string{
			"-placeholder", filepath.ToSlash(rel), meta.Hash, fmt.Sprintf("%d", meta.Size),
		}
		if cfg != "" {
			args = append(args, "-config", cfg)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe, args...)
		if out, err := cmd.CombinedOutput(); err == nil {
			_ = os.Remove(placeholderDiskPath(localRoot, rel))
			_ = os.Remove(placeholderPath(localRoot, rel))
			return nil
		} else {
			fmt.Fprintf(os.Stderr, "cfapi placeholder %s: %v (%s)\n", rel, err, strings.TrimSpace(string(out)))
		}
	}
	lnk := placeholderDiskPath(localRoot, rel)
	if err := os.MkdirAll(filepath.Dir(lnk), 0o755); err != nil {
		return err
	}
	// Remove magic-file legacy placeholder at content path.
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
