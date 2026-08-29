package syncer

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveLocalFolderAbsolute(t *testing.T) {
	cfg := filepath.Join("/appdata/NetoDrive", "netodrive.json")
	got := ResolveLocalFolder(cfg, "C:\\Users\\me\\NetoDrive")
	if runtime.GOOS == "windows" {
		if got != `C:\Users\me\NetoDrive` {
			t.Fatalf("got %q", got)
		}
	} else if got != "/Users/me/NetoDrive" && got != "C:\\Users\\me\\NetoDrive" {
		// filepath.Abs on Linux may leave Windows-style paths unchanged.
		if filepath.IsAbs(got) == false {
			t.Fatalf("expected absolute path, got %q", got)
		}
	}
}

func TestResolveLocalFolderRelativeToConfig(t *testing.T) {
	cfg := filepath.Join("/appdata/NetoDrive", "netodrive.json")
	want := filepath.Clean(filepath.Join("/appdata/NetoDrive", "sync-data"))
	got := ResolveLocalFolder(cfg, "sync-data")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveLocalFolderEmptyUsesDefault(t *testing.T) {
	cfg := filepath.Join("/appdata/NetoDrive", "netodrive.json")
	got := ResolveLocalFolder(cfg, "")
	want := DefaultSyncFolder()
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
