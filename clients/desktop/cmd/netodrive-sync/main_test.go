package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/netodrive/desktop/syncer"
)

func TestLoadConfigLocalFolderAliases(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "netodrive.json")

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "snake_case",
			body: `{"local_folder":"` + filepath.Join(dir, "data") + `"}`,
			want: filepath.Join(dir, "data"),
		},
		{
			name: "camelCase",
			body: `{"LocalFolder":"` + filepath.Join(dir, "other") + `"}`,
			want: filepath.Join(dir, "other"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(cfgPath, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				t.Fatal(err)
			}
			got := syncer.ResolveLocalFolder(cfgPath, cfg.LocalFolder)
			wantAbs, _ := filepath.Abs(tc.want)
			gotAbs, _ := filepath.Abs(got)
			if gotAbs != wantAbs {
				t.Fatalf("got %q want %q", gotAbs, wantAbs)
			}
		})
	}
}

func TestNormalizeConfigDoesNotOverrideOneDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path semantics")
	}
	cfg := Config{LocalFolder: `C:\Users\me\OneDrive\NetoDrive`}
	before := cfg.LocalFolder
	changed := normalizeConfig(&cfg)
	if cfg.LocalFolder != before {
		t.Fatalf("normalizeConfig changed folder from %q to %q", before, cfg.LocalFolder)
	}
	if changed {
		t.Fatalf("expected no config migration for OneDrive path")
	}
}

func TestSaveConfigPreservesLocalFolder(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "netodrive.json")
	want := filepath.Join(dir, "sync")
	cfg := Config{
		ServerURL:   "http://127.0.0.1:8080",
		DeviceID:    "dev",
		LocalFolder: want,
		IntervalSec: 30,
	}
	if err := saveConfig(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		LocalFolder string `json:"local_folder"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	got := raw.LocalFolder
	gotAbs, _ := filepath.Abs(got)
	wantAbs, _ := filepath.Abs(want)
	if gotAbs != wantAbs {
		t.Fatalf("saved %q want %q", gotAbs, wantAbs)
	}
}
