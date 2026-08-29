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

func TestLoadConfigIntervalAliases(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "netodrive.json")

	tests := []struct {
		name string
		body string
		want int
	}{
		{"snake_case", `{"interval_sec":120}`, 120},
		{"camelCase", `{"intervalSec":45}`, 45},
		{"PascalCase", `{"IntervalSec":90}`, 90},
		{"minimum_clamp", `{"interval_sec":0}`, defaultSyncIntervalSec},
		{"one_second", `{"interval_sec":1}`, 1},
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
			if cfg.IntervalSec != tc.want {
				t.Fatalf("IntervalSec = %d want %d", cfg.IntervalSec, tc.want)
			}
		})
	}
}

func TestIntervalFromConfigReloads(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "netodrive.json")
	if err := os.WriteFile(cfgPath, []byte(`{"interval_sec":60}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := intervalFromConfig(cfgPath); got != 60 {
		t.Fatalf("got %d want 60", got)
	}
	if err := os.WriteFile(cfgPath, []byte(`{"interval_sec":15}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := intervalFromConfig(cfgPath); got != 15 {
		t.Fatalf("after edit got %d want 15", got)
	}
}

func TestLoadConfigDoesNotRewriteFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "netodrive.json")
	body := `{
  "server_url": "http://127.0.0.1:8080",
  "local_folder": "C:/Users/henri/NetoDrive",
  "device_id": "dev-1"
}`
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(cfgPath); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("loadConfig rewrote config:\n--- got ---\n%s\n--- want ---\n%s", got, body)
	}
}

func TestPatchConfigFieldsPreservesLocalFolder(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "netodrive.json")
	wantFolder := "C:/Users/henri/NetoDrive"
	body := `{"server_url":"http://127.0.0.1:8080","local_folder":"` + wantFolder + `","device_id":"dev"}`
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := patchConfigFields(cfgPath, map[string]any{"token": "abc123"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		LocalFolder string `json:"local_folder"`
		Token       string `json:"token"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.LocalFolder != wantFolder {
		t.Fatalf("local_folder changed to %q", raw.LocalFolder)
	}
	if raw.Token != "abc123" {
		t.Fatalf("token = %q", raw.Token)
	}
}
