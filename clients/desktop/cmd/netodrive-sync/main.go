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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/netodrive/desktop/syncer"
)

type Config struct {
	ServerURL   string `json:"server_url"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Token       string `json:"token,omitempty"`
	DeviceID    string `json:"device_id"`
	LocalFolder string `json:"local_folder"`
	IntervalSec int    `json:"interval_sec"`
	// OnDemand: arquivos aparecem como placeholder e baixam ao abrir ou fixar.
	OnDemand *bool `json:"on_demand,omitempty"`
	// remote_prefix is ignored (legacy); all devices share the account root tree.
	RemotePrefixLegacy string `json:"remote_prefix,omitempty"`
}

func onDemandEnabled(cfg Config) bool {
	if cfg.OnDemand == nil {
		return true
	}
	return *cfg.OnDemand
}

func main() {
	cfgPath := flag.String("config", defaultConfigPath(), "path to config file")
	once := flag.Bool("once", false, "run a single sync and exit")
	initCfg := flag.Bool("init", false, "write a sample config and exit")
	ui := flag.Bool("ui", false, "open local OneDrive-style control panel and keep syncing")
	openRemote := flag.String("open", "", "download/open a remote path (e.g. docs/arquivo.pdf)")
	pinPath := flag.String("pin", "", "keep path or folder always local (e.g. docs or report.pdf)")
	unpinPath := flag.String("unpin", "", "release path from local pin (back to cloud placeholder)")
	hydratePath := flag.String("hydrate", "", "download path now (file or folder prefix)")
	flag.Parse()

	onDemandDefault := true

	if *initCfg {
		sample := Config{
			ServerURL:   "http://127.0.0.1:8080",
			Username:    "admin",
			Password:    "admin123",
			DeviceID:    uuid.NewString(),
			LocalFolder: defaultSyncFolder(),
			IntervalSec: 30,
			OnDemand:    &onDemandDefault,
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
		fatal(fmt.Errorf("config %s: %w", *cfgPath, err))
	}
	fmt.Printf("Config: %s\n", *cfgPath)
	migrated := false
	if cfg.LocalFolder == "" {
		cfg.LocalFolder = defaultSyncFolder()
		migrated = true
	}
	cfg.LocalFolder = resolveLocalFolder(*cfgPath, cfg.LocalFolder)
	if normalizeConfig(&cfg) {
		migrated = true
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = uuid.NewString()
		migrated = true
	}
	if cfg.IntervalSec <= 0 {
		cfg.IntervalSec = 30
	}
	fmt.Printf("Pasta local de sync: %s\n", cfg.LocalFolder)
	onDemand := onDemandEnabled(cfg)
	if onDemand {
		fmt.Println("Modo: sob demanda (placeholder — baixa ao abrir ou fixar)")
	}

	client := syncer.NewClient(cfg.ServerURL, cfg.Token, cfg.DeviceID)
	if cfg.Token == "" {
		if err := client.Login(cfg.Username, cfg.Password); err != nil {
			fatal(err)
		}
		cfg.Token = client.Token
		migrated = true
	}
	if migrated {
		_ = saveConfig(*cfgPath, cfg)
	}

	if err := os.MkdirAll(cfg.LocalFolder, 0o755); err != nil {
		fatal(err)
	}

	statePath := syncer.DefaultStatePath(*cfgPath)

	if *pinPath != "" {
		target := strings.Trim(strings.ReplaceAll(*pinPath, "\\", "/"), "/")
		if err := syncer.PinLocalPath(client, cfg.LocalFolder, statePath, target, onDemand); err != nil {
			fatal(err)
		}
		fmt.Printf("fixado localmente: %s\n", target)
		return
	}
	if *unpinPath != "" {
		target := strings.Trim(strings.ReplaceAll(*unpinPath, "\\", "/"), "/")
		if err := syncer.UnpinLocalPath(client, cfg.LocalFolder, statePath, target, onDemand); err != nil {
			fatal(err)
		}
		fmt.Printf("liberado (nuvem): %s\n", target)
		return
	}
	if *hydratePath != "" {
		target := strings.Trim(strings.ReplaceAll(*hydratePath, "\\", "/"), "/")
		if err := syncer.HydrateTree(client, cfg.LocalFolder, statePath, target); err != nil {
			fatal(err)
		}
		fmt.Printf("baixado: %s\n", target)
		return
	}

	if *openRemote != "" {
		remote := syncer.ResolveOpenRel(cfg.LocalFolder, *openRemote)
		remote = strings.Trim(strings.ReplaceAll(remote, "\\", "/"), "/")
		if remote == "" {
			fatal(fmt.Errorf("caminho invalido: %q", *openRemote))
		}
		local := filepath.Join(cfg.LocalFolder, filepath.FromSlash(remote))
		fmt.Printf("opening remote %s → %s\n", remote, local)
		if err := syncer.HydratePath(client, cfg.LocalFolder, statePath, remote); err != nil {
			fatal(err)
		}
		if err := openFile(local); err != nil {
			fatal(err)
		}
		return
	}

	run := func() error {
		fmt.Printf("[%s] syncing %s ↔ arvore da conta (raiz)\n", time.Now().Format(time.RFC3339), cfg.LocalFolder)
		if err := syncer.SyncFolder(client, cfg.LocalFolder, statePath, onDemand); err != nil {
			fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
			return err
		}
		fmt.Println("sync ok")
		return nil
	}

	_ = run()

	if *ui {
		startControlPanel(cfg, *cfgPath, client, onDemand, run)
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

func startControlPanel(cfg Config, cfgPath string, client *syncer.Client, onDemand bool, run func() error) {
	statePath := syncer.DefaultStatePath(cfgPath)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(controlPanelHTML(cfg, onDemand)))
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"server_url":   cfg.ServerURL,
			"local_folder": cfg.LocalFolder,
			"interval_sec": cfg.IntervalSec,
			"on_demand":    onDemand,
			"web_url":      cfg.ServerURL,
			"remote_tree":  "arvore da conta (raiz)",
		})
	})
	mux.HandleFunc("/api/hydrate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		path := strings.Trim(body.Path, "/")
		if path == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "path required"})
			return
		}
		err := syncer.HydrateTree(client, cfg.LocalFolder, statePath, path)
		if err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/pin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		path := strings.Trim(body.Path, "/")
		if err := syncer.PinLocalPath(client, cfg.LocalFolder, statePath, path, onDemand); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/unpin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		path := strings.Trim(body.Path, "/")
		if err := syncer.UnpinLocalPath(client, cfg.LocalFolder, statePath, path, onDemand); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true})
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

func controlPanelHTML(cfg Config, onDemand bool) string {
	mode := "Sob demanda — placeholders na pasta; baixe ao abrir ou fixe"
	if !onDemand {
		mode = "Espelho completo — todos os arquivos baixados"
	}
	return `<!doctype html>
<html lang="pt-BR"><head>
<meta charset="utf-8"/>
<title>NetoDrive</title>
<style>
:root{--blue:#0078d4;--bg:#faf9f8;--ink:#242424;--muted:#605e5c;--line:#edebe9;--soft:#deecf9}
*{box-sizing:border-box}body{margin:0;font-family:"Segoe UI","Noto Sans",sans-serif;background:var(--bg);color:var(--ink)}
label{display:block;margin-top:12px;font-size:.92rem}
input{width:100%;margin-top:4px;padding:8px;border:1px solid var(--line)}
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
<p class="muted">` + mode + `</p>
<p class="muted">Pasta local espelhada na <strong>arvore principal da conta</strong> — a mesma vista da web e do Android.</p>
<div class="kv">
<div><b>Servidor</b><span>` + cfg.ServerURL + `</span></div>
<div><b>Pasta local</b><span>` + cfg.LocalFolder + `</span></div>
<div><b>Remoto</b><span>Arvore da conta (raiz)</span></div>
<div><b>Intervalo</b><span>` + fmt.Sprintf("%d s", cfg.IntervalSec) + `</span></div>
</div>
<label>Caminho (arquivo ou pasta)
<input id="path" placeholder="docs/relatorio.pdf ou Galeria"/>
</label>
<div class="row">
<button onclick="syncNow()">Sincronizar agora</button>
<button class="secondary" onclick="act('/api/hydrate')">Baixar agora</button>
<button class="secondary" onclick="act('/api/pin')">Fixar neste PC</button>
<button class="secondary" onclick="act('/api/unpin')">Liberar espaco</button>
<button class="secondary" onclick="post('/api/open-folder')">Abrir pasta</button>
<button class="secondary" onclick="post('/api/open-web')">Abrir NetoDrive na web</button>
</div>
<p id="msg" class="muted"></p>
</div></div>
<script>
async function post(url){const r=await fetch(url,{method:'POST'});const j=await r.json();return j}
async function act(url){const m=document.getElementById('msg');const path=document.getElementById('path').value.trim();
if(!path){m.textContent='Informe um caminho.';return}
const r=await fetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({path})});
const j=await r.json();m.textContent=j.ok?'OK.':('Erro: '+j.error)}
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
		// Fora do OneDrive: Documents costuma ser sincronizado pelo OneDrive no Windows.
		return filepath.Join(home, "NetoDrive")
	}
	return filepath.Join(home, "Documents", "NetoDrive")
}

func isUnderOneDrive(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, env := range []string{"OneDrive", "OneDriveCommercial", "OneDriveConsumer"} {
		base := os.Getenv(env)
		if base == "" {
			continue
		}
		baseAbs, err := filepath.Abs(base)
		if err != nil {
			continue
		}
		if strings.EqualFold(abs, baseAbs) {
			return true
		}
		sep := string(os.PathSeparator)
		if strings.HasPrefix(strings.ToLower(abs), strings.ToLower(baseAbs+sep)) {
			return true
		}
	}
	return false
}

func resolveLocalFolder(cfgPath, folder string) string {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return defaultSyncFolder()
	}
	if filepath.IsAbs(folder) {
		abs, _ := filepath.Abs(folder)
		return abs
	}
	// Caminhos relativos são relativos ao arquivo de config, não ao CWD
	base := filepath.Dir(cfgPath)
	return filepath.Clean(filepath.Join(base, folder))
}

func looksLikeRepoRoot(dir string) bool {
	for _, marker := range []string{".git", filepath.Join("server", "go.mod"), filepath.Join("clients", "desktop")} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(b, &cfg)
	return cfg, err
}

// normalizeConfig clears legacy settings and fixes common misconfiguration.
func normalizeConfig(cfg *Config) bool {
	changed := cfg.RemotePrefixLegacy != ""
	cfg.RemotePrefixLegacy = ""

	home, _ := os.UserHomeDir()
	repoClone := filepath.Join(home, "NetoDrive")
	folder := strings.TrimSpace(cfg.LocalFolder)
	if folder == "" {
		cfg.LocalFolder = defaultSyncFolder()
		return true
	}
	abs, err := filepath.Abs(folder)
	if err != nil {
		abs = folder
	}
	// Migra quem apontou sync para a pasta do clone git do projeto
	if abs == repoClone || looksLikeRepoRoot(abs) {
		cfg.LocalFolder = defaultSyncFolder()
		fmt.Fprintf(os.Stderr, "Aviso: local_folder apontava para o projeto git (%s).\n", abs)
		fmt.Fprintf(os.Stderr, "         Usando pasta de dados: %s\n", cfg.LocalFolder)
		return true
	}
	if runtime.GOOS == "windows" && isUnderOneDrive(abs) {
		cfg.LocalFolder = defaultSyncFolder()
		fmt.Fprintf(os.Stderr, "Aviso: local_folder estava dentro do OneDrive (%s).\n", abs)
		fmt.Fprintf(os.Stderr, "         CFAPI nao funciona dentro do OneDrive. Usando: %s\n", cfg.LocalFolder)
		return true
	}
	if abs != cfg.LocalFolder {
		cfg.LocalFolder = abs
		changed = true
	}
	return changed
}

func saveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	normalizeConfig(&cfg)
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
