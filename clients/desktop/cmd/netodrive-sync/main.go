package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	cfgPath := flag.String("config", defaultConfigPath(), "path to config file")
	once := flag.Bool("once", false, "run a single sync and exit")
	initCfg := flag.Bool("init", false, "write a sample config and exit")
	ui := flag.Bool("ui", false, "open local OneDrive-style control panel and keep syncing")
	openRemote := flag.String("open", "", "download/open a remote path (e.g. PC/docs/a.txt)")
	flag.Parse()

	if *initCfg {
		sample := Config{
			ServerURL:    "http://127.0.0.1:8080",
			Username:     "admin",
			Password:     "admin123",
			DeviceID:     uuid.NewString(),
			LocalFolder:  defaultSyncFolder(),
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

	if *openRemote != "" {
		local := filepath.Join(cfg.LocalFolder, filepath.FromSlash(*openRemote))
		// If under remote prefix, strip when mapping; otherwise use as relative under local
		if cfg.RemotePrefix != "" {
			// remote path is absolute on server; download to local mirror
			rel := *openRemote
			prefix := cfg.RemotePrefix + "/"
			if len(rel) >= len(prefix) && rel[:len(prefix)] == prefix {
				rel = rel[len(prefix):]
			} else if rel == cfg.RemotePrefix {
				fatal(fmt.Errorf("path is a folder, not a file"))
			}
			local = filepath.Join(cfg.LocalFolder, filepath.FromSlash(rel))
		}
		fmt.Printf("opening remote %s → %s\n", *openRemote, local)
		if err := client.Download(*openRemote, local); err != nil {
			fatal(err)
		}
		if err := openFile(local); err != nil {
			fatal(err)
		}
		return
	}

	run := func() error {
		fmt.Printf("[%s] syncing %s ↔ %s/\n", time.Now().Format(time.RFC3339), cfg.LocalFolder, cfg.RemotePrefix)
		if err := syncer.SyncFolder(client, cfg.LocalFolder, cfg.RemotePrefix); err != nil {
			fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
			return err
		}
		fmt.Println("sync ok")
		return nil
	}

	_ = run()

	if *ui {
		startControlPanel(cfg, client, run)
		return
	}

	if *once {
		return
	}
	t := time.NewTicker(time.Duration(cfg.IntervalSec) * time.Second)
	defer t.Stop()
	for range t.C {
		_ = run()
	}
}

func startControlPanel(cfg Config, client *syncer.Client, run func() error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(controlPanelHTML(cfg)))
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"server_url":    cfg.ServerURL,
			"local_folder":  cfg.LocalFolder,
			"remote_prefix": cfg.RemotePrefix,
			"interval_sec":  cfg.IntervalSec,
			"web_url":       cfg.ServerURL,
		})
	})
	mux.HandleFunc("/api/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		err := run()
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/open-folder", func(w http.ResponseWriter, r *http.Request) {
		_ = openFile(cfg.LocalFolder)
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/open-web", func(w http.ResponseWriter, r *http.Request) {
		_ = openURL(cfg.ServerURL)
		writeJSON(w, map[string]any{"ok": true})
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal(err)
	}
	addr := "http://" + ln.Addr().String()
	fmt.Printf("Painel NetoDrive: %s\n", addr)
	_ = openURL(addr)

	go func() {
		t := time.NewTicker(time.Duration(cfg.IntervalSec) * time.Second)
		defer t.Stop()
		for range t.C {
			_ = run()
		}
	}()

	if err := http.Serve(ln, mux); err != nil {
		fatal(err)
	}
}

func controlPanelHTML(cfg Config) string {
	return `<!doctype html>
<html lang="pt-BR"><head>
<meta charset="utf-8"/>
<title>NetoDrive</title>
<style>
:root{--blue:#0078d4;--bg:#faf9f8;--ink:#242424;--muted:#605e5c;--line:#edebe9;--soft:#deecf9}
*{box-sizing:border-box}body{margin:0;font-family:"Segoe UI","Noto Sans",sans-serif;background:var(--bg);color:var(--ink)}
.top{background:var(--blue);color:#fff;padding:12px 20px;display:flex;align-items:center;gap:10px;font-weight:600}
.mark{width:28px;height:28px;border-radius:6px;background:rgba(255,255,255,.2);display:grid;place-items:center}
.wrap{max-width:720px;margin:24px auto;padding:0 16px}
.card{background:#fff;border:1px solid var(--line);padding:20px;box-shadow:0 1.6px 3.6px rgba(0,0,0,.08)}
h1{margin:0 0 6px;font-size:1.5rem;font-weight:600}.muted{color:var(--muted);font-size:.92rem}
.row{display:flex;flex-wrap:wrap;gap:8px;margin-top:16px}
button{border:1px solid transparent;background:var(--blue);color:#fff;font-weight:600;padding:10px 14px;border-radius:2px;cursor:pointer}
button.secondary{background:#fff;color:var(--ink);border-color:#8a8886}
button:hover{filter:brightness(.96)}
.kv{margin-top:16px;display:grid;gap:8px}.kv div{display:grid;grid-template-columns:140px 1fr;gap:8px;font-size:.92rem}
.kv b{color:var(--muted);font-weight:600}
#msg{margin-top:12px;min-height:1.2em}
</style></head><body>
<div class="top"><div class="mark">N</div>NetoDrive — Sincronização</div>
<div class="wrap"><div class="card">
<h1>Seus arquivos neste PC</h1>
<p class="muted">Como o OneDrive: uma pasta local sincronizada com o servidor Linux.</p>
<div class="kv">
<div><b>Servidor</b><span>` + cfg.ServerURL + `</span></div>
<div><b>Pasta local</b><span>` + cfg.LocalFolder + `</span></div>
<div><b>Remoto</b><span>` + cfg.RemotePrefix + `/</span></div>
<div><b>Intervalo</b><span>` + fmt.Sprintf("%d s", cfg.IntervalSec) + `</span></div>
</div>
<div class="row">
<button onclick="syncNow()">Sincronizar agora</button>
<button class="secondary" onclick="post('/api/open-folder')">Abrir pasta</button>
<button class="secondary" onclick="post('/api/open-web')">Abrir NetoDrive na web</button>
</div>
<p id="msg" class="muted"></p>
</div></div>
<script>
async function post(url){const r=await fetch(url,{method:'POST'});const j=await r.json();return j}
async function syncNow(){const m=document.getElementById('msg');m.textContent='Sincronizando…';
try{const j=await post('/api/sync');m.textContent=j.ok?'Sincronização concluída.':('Erro: '+j.error)}
catch(e){m.textContent=String(e)}}
</script></body></html>`
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func openFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func defaultConfigPath() string {
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Roaming", "NetoDrive", "netodrive.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "netodrive", "netodrive.json")
}

func defaultSyncFolder() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "NetoDrive")
	}
	return filepath.Join(home, "NetoDrive")
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
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
