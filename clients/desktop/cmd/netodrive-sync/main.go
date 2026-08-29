package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/netodrive/desktop/syncer"
)

type Config struct {
	ServerURL    string `json:"server_url"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Token        string `json:"token,omitempty"`
	DeviceID     string `json:"device_id"`
	LocalFolder  string `json:"local_folder"`
	RemotePrefix string `json:"remote_prefix"`
	IntervalSec  int    `json:"interval_sec"`
}

func main() {
	cfgPath := flag.String("config", "netodrive.json", "path to config file")
	once := flag.Bool("once", false, "run a single sync and exit")
	initCfg := flag.Bool("init", false, "write a sample config and exit")
	flag.Parse()

	if *initCfg {
		sample := Config{
			ServerURL:    "http://127.0.0.1:8080",
			Username:     "admin",
			Password:     "admin123",
			DeviceID:     uuid.NewString(),
			LocalFolder:  "./sync",
			RemotePrefix: "PC",
			IntervalSec:  30,
		}
		b, _ := json.MarshalIndent(sample, "", "  ")
		if err := os.WriteFile(*cfgPath, b, 0o600); err != nil {
			fatal(err)
		}
		fmt.Printf("wrote %s\n", *cfgPath)
		return
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fatal(err)
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = uuid.NewString()
	}
	if cfg.IntervalSec <= 0 {
		cfg.IntervalSec = 30
	}
	if err := os.MkdirAll(cfg.LocalFolder, 0o755); err != nil {
		fatal(err)
	}

	client := syncer.NewClient(cfg.ServerURL, cfg.Token, cfg.DeviceID)
	if cfg.Token == "" {
		if err := client.Login(cfg.Username, cfg.Password); err != nil {
			fatal(err)
		}
		cfg.Token = client.Token
		_ = saveConfig(*cfgPath, cfg)
	}

	run := func() {
		fmt.Printf("[%s] syncing %s ↔ %s/\n", time.Now().Format(time.RFC3339), cfg.LocalFolder, cfg.RemotePrefix)
		if err := syncer.SyncFolder(client, cfg.LocalFolder, cfg.RemotePrefix); err != nil {
			fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
			return
		}
		fmt.Println("sync ok")
	}

	run()
	if *once {
		return
	}
	t := time.NewTicker(time.Duration(cfg.IntervalSec) * time.Second)
	defer t.Stop()
	for range t.C {
		run()
	}
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(b, &cfg)
	if cfg.LocalFolder != "" {
		cfg.LocalFolder, _ = filepath.Abs(cfg.LocalFolder)
	}
	return cfg, err
}

func saveConfig(path string, cfg Config) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
