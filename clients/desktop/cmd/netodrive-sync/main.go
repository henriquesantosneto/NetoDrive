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
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/netodrive/desktop/syncer"
)

const buildVersion = "fast-path-cfapi-v18"

const (
	minSyncIntervalSec     = 1
	maxSyncIntervalSec     = 86400
	defaultSyncIntervalSec = 30
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
	// IsRepoRoot: true when local_folder is the git clone (evita Stat sob CFAPI). Ex.: true
	IsRepoRoot *bool `json:"is_repo_root,omitempty"`
}

var (
	syncMu       sync.Mutex
	syncRunning  bool
	syncStarted  time.Time
	syncFinished time.Time
)

const syncTimeout = 90 * time.Second

func onDemandEnabled(cfg Config) bool {
	if cfg.OnDemand == nil {
		return true
	}
	return *cfg.OnDemand
}

func main() {
	cfgPath := flag.String("config", defaultConfigPath(), "path to config file")
	showVersion := flag.Bool("version", false, "print build version and exit")
	once := flag.Bool("once", false, "run a single sync and exit")
	initCfg := flag.Bool("init", false, "write a sample config and exit")
	ui := flag.Bool("ui", false, "open local OneDrive-style control panel and keep syncing")
	openRemote := flag.String("open", "", "download/open a remote path (e.g. docs/arquivo.pdf)")
	pinPath := flag.String("pin", "", "keep path or folder always local (e.g. docs or report.pdf)")
	unpinPath := flag.String("unpin", "", "release path from local pin (back to cloud placeholder)")
	hydratePath := flag.String("hydrate", "", "download path now (file or folder prefix)")
	printLocal := flag.Bool("print-local-folder", false, "print resolved local_folder and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(buildVersion)
		return
	}

	onDemandDefault := true

	if *initCfg {
		sample := Config{
			ServerURL:   "http://127.0.0.1:8080",
			Username:    "admin",
			Password:    "admin123",
			DeviceID:    uuid.NewString(),
			LocalFolder: syncer.DefaultSyncFolder(),
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

	if *printLocal {
		folder, err := loadResolvedLocalFolder(*cfgPath)
		if err != nil {
			fatal(err)
		}
		fmt.Println(folder)
		return
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fatal(fmt.Errorf("config %s: %w", *cfgPath, err))
	}
	fmt.Printf("Config: %s\n", *cfgPath)
	if strings.TrimSpace(cfg.LocalFolder) == "" {
		cfg.LocalFolder = syncer.DefaultSyncFolder()
		fmt.Fprintf(os.Stderr, "Aviso: local_folder vazio no JSON; usando %s (edite o arquivo para persistir).\n", cfg.LocalFolder)
	}
	cfg.LocalFolder = syncer.ResolveLocalFolder(*cfgPath, cfg.LocalFolder)
	normalizeConfig(&cfg)
	statePath := syncer.DefaultStatePath(*cfgPath)
	syncer.PrepareRepoRootCache(cfg.LocalFolder, statePath, cfg.IsRepoRoot)
	warnLocalFolderIssues(cfg.LocalFolder, cfg.IsRepoRoot)
	if cfg.DeviceID == "" {
		cfg.DeviceID = uuid.NewString()
		if err := patchConfigFields(*cfgPath, map[string]any{"device_id": cfg.DeviceID}); err != nil {
			fmt.Fprintf(os.Stderr, "Aviso: nao foi possivel gravar device_id no config: %v\n", err)
		}
	}
	if cfg.IntervalSec <= 0 {
		cfg.IntervalSec = defaultSyncIntervalSec
	}
	cfg.IntervalSec = normalizeIntervalSec(cfg.IntervalSec)
	fmt.Printf("Pasta local de sync: %s\n", cfg.LocalFolder)
	fmt.Fprintf(os.Stderr, "NetoDrive sync engine: %s\n", buildVersion)
	cfapiActive := syncer.CfapiProviderInstalled()
	if cfapiActive {
		fmt.Fprintf(os.Stderr, "CFAPI ativo: sync automatico a cada %ds (manifest, deletes locais, placeholders)\n", cfg.IntervalSec)
	}
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
		if err := patchConfigFields(*cfgPath, map[string]any{"token": cfg.Token}); err != nil {
			fmt.Fprintf(os.Stderr, "Aviso: nao foi possivel gravar token no config: %v\n", err)
		}
	}

	if err := os.MkdirAll(cfg.LocalFolder, 0o755); err != nil {
		fatal(err)
	}

	if err := client.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "AVISO: servidor offline em %s (%v)\n", cfg.ServerURL, err)
		fmt.Fprintf(os.Stderr, "  Sync falha ate o servidor subir. Edite server_url em %s\n", *cfgPath)
	}

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
		syncMu.Lock()
		if syncRunning {
			if time.Since(syncStarted) > syncTimeout+time.Minute {
				fmt.Fprintf(os.Stderr, "aviso: sync anterior parece travado; liberando para nova tentativa\n")
				syncRunning = false
			} else {
				syncMu.Unlock()
				return fmt.Errorf("sync ja em andamento")
			}
		}
		syncRunning = true
		syncStarted = time.Now()
		syncMu.Unlock()
		defer func() {
			syncMu.Lock()
			syncRunning = false
			syncFinished = time.Now()
			syncMu.Unlock()
		}()

		fmt.Fprintf(os.Stderr, "[%s] syncing %s ↔ arvore da conta (raiz)\n", time.Now().Format(time.RFC3339), cfg.LocalFolder)

		fmt.Fprintln(os.Stderr, "sync: verificando alteracoes remotas...")
		if ok, qerr := syncer.TryQuickSync(client, statePath, cfg.LocalFolder); qerr != nil {
			fmt.Fprintf(os.Stderr, "sync: quick-check indisponivel (%v); sync completo\n", qerr)
		} else if ok {
			fmt.Fprintln(os.Stderr, "sync: sem alteracoes remotas (quick)")
			fmt.Fprintln(os.Stderr, "sync ok")
			return nil
		}

		fmt.Fprintln(os.Stderr, "sync: chamando engine...")
		errCh := make(chan error, 1)
		go func() {
			var err error
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("sync panic: %v", r)
				}
				errCh <- err
			}()
			err = syncer.SyncFolder(client, cfg.LocalFolder, statePath, onDemand)
		}()
		var err error
		select {
		case err = <-errCh:
		case <-time.After(syncTimeout):
			fmt.Fprintf(os.Stderr, "sync timeout apos %s - use Liberar sync travado\n", syncTimeout)
			return fmt.Errorf("sync timeout apos %s", syncTimeout)
		}

		if err != nil {
			if syncer.IsConnectionError(err) {
				fmt.Fprintf(os.Stderr, "sync error: servidor inacessivel (%s): %v\n", cfg.ServerURL, err)
				fmt.Fprintf(os.Stderr, "  Inicie o servidor NetoDrive ou corrija server_url em %s\n", *cfgPath)
			} else {
				fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
			}
			return err
		}
		fmt.Fprintln(os.Stderr, "sync ok")
		return nil
	}

	if *ui {
		startControlPanel(cfg, *cfgPath, client, onDemand, cfapiActive, run)
		return
	}

	if *once {
		if err := run(); err != nil {
			fatal(err)
		}
		return
	}
	if *ui {
		startControlPanel(cfg, *cfgPath, client, onDemand, cfapiActive, run)
		return
	}
	startBackgroundSync(*cfgPath, run)
	select {}
}

func normalizeIntervalSec(sec int) int {
	if sec <= 0 {
		sec = defaultSyncIntervalSec
	}
	if sec < minSyncIntervalSec {
		sec = minSyncIntervalSec
	}
	if sec > maxSyncIntervalSec {
		sec = maxSyncIntervalSec
	}
	return sec
}

func intervalFromConfig(cfgPath string) int {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return defaultSyncIntervalSec
	}
	return normalizeIntervalSec(cfg.IntervalSec)
}

func startBackgroundSync(cfgPath string, run func() error) {
	interval := time.Duration(intervalFromConfig(cfgPath)) * time.Second
	fmt.Fprintf(os.Stderr, "sync automatico: a cada %s (interval_sec no config)\n", interval)
	go func() {
		time.Sleep(3 * time.Second)
		if err := run(); err != nil && !strings.Contains(err.Error(), "sync ja em andamento") {
			fmt.Fprintf(os.Stderr, "sync inicial: %v\n", err)
		}
	}()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			<-ticker.C
			if newInterval := time.Duration(intervalFromConfig(cfgPath)) * time.Second; newInterval != interval {
				interval = newInterval
				ticker.Stop()
				ticker = time.NewTicker(interval)
				fmt.Fprintf(os.Stderr, "sync: intervalo atualizado para %s\n", interval)
			}
			if err := run(); err != nil && strings.Contains(err.Error(), "sync ja em andamento") {
				continue
			}
		}
	}()
}

func startControlPanel(cfg Config, cfgPath string, client *syncer.Client, onDemand bool, cfapiActive bool, run func() error) {
	statePath := syncer.DefaultStatePath(cfgPath)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(controlPanelHTML(cfg, onDemand, cfapiActive)))
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		syncMu.Lock()
		running := syncRunning
		started := syncStarted
		finished := syncFinished
		syncMu.Unlock()
		stuck := running && time.Since(started) > 15*time.Second
		serverOK := !running && client.Ping() == nil
		if running {
			serverOK = true
		}
		intervalSec := intervalFromConfig(cfgPath)
		writeJSON(w, map[string]any{
			"server_url":         cfg.ServerURL,
			"local_folder":       cfg.LocalFolder,
			"interval_sec":       intervalSec,
			"auto_sync":          true,
			"cfapi_active":       cfapiActive,
			"engine_version":     buildVersion,
			"on_demand":          onDemand,
			"web_url":            cfg.ServerURL,
			"remote_tree":        "arvore da conta (raiz)",
			"server_online":      serverOK,
			"sync_running":       running,
			"sync_stuck":         stuck,
			"sync_finished_at":   finished.UnixMilli(),
			"sync_started_at":    started.UnixMilli(),
		})
	})
	mux.HandleFunc("/api/sync-reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		syncMu.Lock()
		syncRunning = false
		syncMu.Unlock()
		syncer.InvalidateStateCache(statePath)
		writeJSON(w, map[string]any{"ok": true})
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
		syncMu.Lock()
		busy := syncRunning
		syncMu.Unlock()
		if busy {
			writeJSON(w, map[string]any{"ok": false, "error": "sync ja em andamento"})
			return
		}
		go func() {
			if err := run(); err != nil {
				fmt.Fprintf(os.Stderr, "sync: %v\n", err)
			}
		}()
		writeJSON(w, map[string]any{"ok": true, "started": true})
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

	startBackgroundSync(cfgPath, run)

	if err := http.Serve(ln, mux); err != nil {
		fatal(err)
	}
}

func controlPanelIntervalLabel(intervalSec int, cfapiActive bool) string {
	intervalSec = normalizeIntervalSec(intervalSec)
	label := fmt.Sprintf("%d s", intervalSec)
	if cfapiActive {
		return label + " (CFAPI)"
	}
	return label
}

func controlPanelHTML(cfg Config, onDemand bool, cfapiActive bool) string {
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
#srv{margin-top:12px;padding:10px;border-radius:4px;font-size:.92rem}
#srv.ok{background:#dff6dd;border:1px solid #107c10}
#srv.bad{background:#fde7e9;border:1px solid #a80000}
</style></head><body>
<div class="top"><div class="mark">N</div>NetoDrive — Sincronização</div>
<div class="wrap"><div class="card">
<h1>Seus arquivos neste PC</h1>
<p class="muted">` + mode + `</p>
<p class="muted">Pasta local espelhada na <strong>arvore principal da conta</strong> — a mesma vista da web e do Android.</p>
<div id="srv" class="bad">Verificando servidor…</div>
<div class="kv">
<div><b>Servidor</b><span>` + cfg.ServerURL + `</span></div>
<div><b>Pasta local</b><span>` + cfg.LocalFolder + `</span></div>
<div><b>Remoto</b><span>Arvore da conta (raiz)</span></div>
<div><b>Intervalo</b><span id="interval">` + controlPanelIntervalLabel(cfg.IntervalSec, cfapiActive) + `</span></div>
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
<button class="secondary" onclick="resetSync()">Liberar sync travado</button>
<button class="secondary" onclick="post('/api/open-web')">Abrir NetoDrive na web</button>
</div>
<p id="msg" class="muted"></p>
</div></div>
<script>
async function post(url,opts){const r=await fetch(url,Object.assign({method:'POST'},opts||{}));return r.json()}
async function act(url){const m=document.getElementById('msg');const path=document.getElementById('path').value.trim();
if(!path){m.textContent='Informe um caminho.';return}
const r=await fetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({path})});
const j=await r.json();m.textContent=j.ok?'OK.':('Erro: '+j.error)}
async function syncNow(){
const m=document.getElementById('msg');const btn=document.querySelector('button');
m.textContent='Sincronizando…';if(btn)btn.disabled=true;
let watchStarted=0;
try{
const j=await post('/api/sync');
if(!j.ok){m.textContent='Erro: '+(j.error||'falhou');return}
for(let i=0;i<60;i++){
await new Promise(r=>setTimeout(r,500));
const s=await fetch('/api/status').then(r=>r.json());
if(s.sync_running&&s.sync_started_at)watchStarted=Math.max(watchStarted,s.sync_started_at);
if(s.sync_stuck){m.textContent='Sync travado — clique Liberar sync travado.';break}
if(!s.sync_running){
if(watchStarted===0||s.sync_finished_at>=watchStarted){
m.textContent='Sincronizacao concluida.';break}}
if(i===59)m.textContent='Demorou mais que 30s — veja o console.'}
}catch(e){m.textContent=String(e)}
finally{if(btn)btn.disabled=false;refreshServer()}}
async function refreshServer(){
const el=document.getElementById('srv');
const iv=document.getElementById('interval');
try{
const s=await fetch('/api/status').then(r=>r.json());
if(s.server_online){el.className='ok';el.textContent='Servidor online: '+s.server_url}
else{el.className='bad';el.textContent='Servidor OFFLINE em '+s.server_url+' — inicie o NetoDrive server ou corrija server_url no netodrive.json'}
if(s.sync_stuck){el.className='bad';el.textContent+=' — SYNC TRAVADO: feche e reabra o NetoDrive Sync ou clique Liberar sync'}
else if(s.sync_running){el.textContent+=' (sync em andamento)'}
if(iv){iv.textContent=(s.interval_sec||30)+' s'+(s.cfapi_active?' (CFAPI)':'')}
}catch(e){el.className='bad';el.textContent='Nao foi possivel verificar o servidor.'}}
async function resetSync(){const m=document.getElementById('msg');await post('/api/sync-reset');m.textContent='Sync liberado. Tente sincronizar de novo.';refreshServer()}
refreshServer();setInterval(refreshServer,15000);
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

func loadResolvedLocalFolder(cfgPath string) (string, error) {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.LocalFolder) == "" {
		cfg.LocalFolder = syncer.DefaultSyncFolder()
	}
	cfg.LocalFolder = syncer.ResolveLocalFolder(cfgPath, cfg.LocalFolder)
	normalizeConfig(&cfg)
	return cfg.LocalFolder, nil
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	fixed := syncer.FixJSONWindowsPaths(b)
	if err := json.Unmarshal(fixed, &cfg); err != nil {
		if lf, ok := syncer.ExtractLocalFolderFromBrokenJSON(b); ok {
			cfg.LocalFolder = lf
		} else {
			return cfg, fmt.Errorf("config %s: JSON invalido (local_folder precisa de \\\\ ou /): %w", path, err)
		}
	}
	if strings.TrimSpace(cfg.LocalFolder) == "" {
		var raw map[string]json.RawMessage
		if json.Unmarshal(fixed, &raw) == nil {
			for _, key := range []string{"LocalFolder", "localFolder"} {
				v, ok := raw[key]
				if !ok {
					continue
				}
				var s string
				if json.Unmarshal(v, &s) == nil && strings.TrimSpace(s) != "" {
					cfg.LocalFolder = s
					break
				}
			}
		}
	}
	if cfg.IntervalSec <= 0 {
		var raw map[string]json.RawMessage
		if json.Unmarshal(fixed, &raw) == nil {
			for _, key := range []string{"intervalSec", "IntervalSec", "sync_interval_sec"} {
				v, ok := raw[key]
				if !ok {
					continue
				}
				var n int
				if json.Unmarshal(v, &n) == nil && n > 0 {
					cfg.IntervalSec = n
					break
				}
			}
		}
	}
	cfg.IntervalSec = normalizeIntervalSec(cfg.IntervalSec)
	return cfg, nil
}

// patchConfigFields updates only the given keys, preserving the rest of the file.
func patchConfigFields(path string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fixed := syncer.FixJSONWindowsPaths(b)
	var root map[string]json.RawMessage
	if err := json.Unmarshal(fixed, &root); err != nil {
		return fmt.Errorf("config invalido (nao alterado): %w", err)
	}
	for k, v := range fields {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		root[k] = raw
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o600)
}

// normalizeConfig clears legacy settings and canonicalizes paths without overriding user choice.
func normalizeConfig(cfg *Config) bool {
	changed := cfg.RemotePrefixLegacy != ""
	cfg.RemotePrefixLegacy = ""

	folder := strings.TrimSpace(cfg.LocalFolder)
	if folder == "" {
		cfg.LocalFolder = syncer.DefaultSyncFolder()
		return true
	}
	abs, err := filepath.Abs(folder)
	if err != nil {
		abs = folder
	}
	if abs != cfg.LocalFolder {
		cfg.LocalFolder = abs
		changed = true
	}
	return changed
}

func suggestedAlternateSyncFolder(home, current string) string {
	currentAbs, err := filepath.Abs(current)
	if err != nil {
		currentAbs = current
	}
	for _, candidate := range []string{
		filepath.Join(home, "NetoDriveSync"),
		filepath.Join(home, "NetoDriveData"),
		filepath.Join(home, "Documents", "NetoDriveData"),
	} {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if !strings.EqualFold(abs, currentAbs) {
			return candidate
		}
	}
	return filepath.Join(home, "NetoDriveSync")
}

func warnLocalFolderIssues(abs string, cfgIsRepo *bool) {
	home, _ := os.UserHomeDir()
	isRepo := cfgIsRepo != nil && *cfgIsRepo
	if !isRepo && !syncer.CfapiProviderInstalled() {
		isRepo = syncer.IsLikelyRepoRoot(abs)
	}
	if isRepo {
		fmt.Fprintf(os.Stderr, "Aviso: local_folder e o clone git (%s).\n", abs)
		fmt.Fprintf(os.Stderr, "         Pastas clients/server/web nao serao enviadas ao servidor.\n")
		fmt.Fprintf(os.Stderr, "         Recomendado: pasta so de dados, ex. %s\n", suggestedAlternateSyncFolder(home, abs))
		fmt.Fprintf(os.Stderr, "         Com CFAPI ativo, adicione ao netodrive.json: \"is_repo_root\": true\n")
	}
	if runtime.GOOS == "windows" && syncer.IsUnderOneDrive(abs) {
		fmt.Fprintf(os.Stderr, "Aviso: local_folder esta dentro do OneDrive (%s).\n", abs)
		fmt.Fprintf(os.Stderr, "         O registro CFAPI nativo pode falhar; prefira pasta fora do OneDrive.\n")
	}
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
