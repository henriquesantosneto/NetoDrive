//go:build windows

package syncer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func writePlatformPlaceholder(localRoot, rel string, meta placeholderMeta) error {
	if err := writePlaceholderMeta(localRoot, rel, meta); err != nil {
		return err
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
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create shortcut %s: %w (%s)", lnk, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isPlatformPlaceholder(path string) bool {
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
