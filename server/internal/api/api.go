package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/netodrive/server/internal/auth"
	"github.com/netodrive/server/internal/config"
	"github.com/netodrive/server/internal/store"
)

type Server struct {
	Cfg   config.Config
	Store *store.Store
	Auth  *auth.Service
}

func New(cfg config.Config, st *store.Store, a *auth.Service) *Server {
	return &Server{Cfg: cfg, Store: st, Auth: a}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/auth/me", s.withAuth(s.handleMe))

	mux.HandleFunc("/api/files", s.withAuth(s.handleFiles))
	mux.HandleFunc("/api/files/", s.withAuth(s.handleFileByPath))
	mux.HandleFunc("/api/sync/manifest", s.withAuth(s.handleManifest))
	mux.HandleFunc("/api/sync/changes", s.withAuth(s.handleChanges))
	mux.HandleFunc("/api/sync/upload", s.withAuth(s.handleUpload))
	mux.HandleFunc("/api/sync/download/", s.withAuth(s.handleDownload))
	mux.HandleFunc("/api/open/", s.withAuth(s.handleOpen))
	mux.HandleFunc("/api/gallery", s.withAuth(s.handleGallery))
	mux.HandleFunc("/api/gallery/sync", s.withAuth(s.handleGallerySync))
	mux.HandleFunc("/api/trash", s.withAuth(s.handleTrash))
	mux.HandleFunc("/api/trash/", s.withAuth(s.handleTrashItem))
	s.registerBulkRoutes(mux)

	// Static web UI (optional)
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	webRoots := []string{
		filepath.Join("web", "dist"),
		filepath.Join("..", "web", "dist"),
		filepath.Join("..", "..", "..", "web", "dist"),
		filepath.Join("/app", "web", "dist"),
	}
	if exeDir != "" {
		webRoots = append([]string{
			filepath.Join(exeDir, "web", "dist"),
			filepath.Join(exeDir, "..", "web", "dist"),
			filepath.Join(exeDir, "..", "..", "..", "web", "dist"),
		}, webRoots...)
	}
	var webRoot string
	for _, candidate := range webRoots {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			webRoot = candidate
			break
		}
	}
	if webRoot != "" {
		fs := http.FileServer(http.Dir(webRoot))
		mux.Handle("/", spaFallback(webRoot, fs))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"NetoDrive","status":"ok"}`))
		})
	}

	return withCORS(mux)
}

func spaFallback(root string, fs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		p := filepath.Join(root, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Device-Id, X-File-Path, X-File-Mime, X-File-Mtime, X-Gallery-Key, X-Width, X-Height, X-Taken-At")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type ctxKey string

const claimsKey ctxKey = "claims"

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := auth.Bearer(r)
		if token == "" {
			writeErr(w, http.StatusUnauthorized, "missing token")
			return
		}
		claims, err := s.Auth.Parse(token)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		r = r.WithContext(withClaims(r.Context(), claims))
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "netodrive"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	u, err := s.Store.GetUserByUsername(body.Username)
	if err != nil || !auth.CheckPassword(u.PasswordHash, body.Password) {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.Auth.Issue(u.ID, u.Username, 30*24*time.Hour)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"user_id":  u.ID,
		"username": u.Username,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	writeJSON(w, http.StatusOK, map[string]any{"user_id": c.UserID, "username": c.Username})
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	parent := strings.Trim(r.URL.Query().Get("path"), "/")
	files, err := s.Store.ListDir(c.UserID, parent)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if files == nil {
		files = []store.FileMeta{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": parent, "files": files})
}

func (s *Server) handleFileByPath(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	p := strings.TrimPrefix(r.URL.Path, "/api/files/")
	p = path.Clean("/" + p)
	p = strings.TrimPrefix(p, "/")
	if p == "." || p == "" {
		writeErr(w, http.StatusBadRequest, "invalid path")
		return
	}

	switch r.Method {
	case http.MethodGet:
		f, err := s.Store.GetFileByPath(c.UserID, p)
		if err != nil || f.Deleted {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, http.StatusOK, f)
	case http.MethodDelete:
		f, err := s.Store.SoftDelete(c.UserID, p)
		if err != nil {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, http.StatusOK, f)
	case http.MethodPost:
		var body struct {
			IsDir bool `json:"is_dir"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !body.IsDir {
			writeErr(w, http.StatusBadRequest, "only directory create supported here")
			return
		}
		deviceID := r.Header.Get("X-Device-Id")
		if err := s.Store.EnsureParentDirs(c.UserID, p+"/placeholder", deviceID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		meta := &store.FileMeta{
			UserID:   c.UserID,
			Path:     p,
			Name:     filepath.Base(p),
			IsDir:    true,
			Mime:     "inode/directory",
			MTime:    time.Now().UTC(),
			DeviceID: deviceID,
		}
		if err := s.Store.UpsertFile(meta); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, meta)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	files, err := s.Store.ListAllActive(c.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	type entry struct {
		Path string `json:"path"`
		Hash string `json:"hash"`
		Size int64  `json:"size"`
		IsDir bool  `json:"is_dir"`
		MTime time.Time `json:"mtime"`
		Version int64 `json:"version"`
	}
	out := make([]entry, 0, len(files))
	var maxVer int64
	for _, f := range files {
		out = append(out, entry{Path: f.Path, Hash: f.Hash, Size: f.Size, IsDir: f.IsDir, MTime: f.MTime, Version: f.Version})
		if f.Version > maxVer {
			maxVer = f.Version
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": maxVer, "files": out})
}

func (s *Server) handleChanges(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	changes, err := s.Store.ChangesSince(c.UserID, since, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if changes == nil {
		changes = []store.Change{}
	}
	var cursor int64 = since
	if len(changes) > 0 {
		cursor = changes[len(changes)-1].ID
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": changes, "cursor": cursor})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	c := claimsFrom(r)
	filePath := strings.Trim(r.Header.Get("X-File-Path"), "/")
	if filePath == "" {
		filePath = strings.Trim(r.URL.Query().Get("path"), "/")
	}
	if filePath == "" || strings.Contains(filePath, "..") {
		writeErr(w, http.StatusBadRequest, "invalid path")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes)
	tmp, err := os.CreateTemp(s.Cfg.DataDir, "upload-*")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "temp file error")
		return
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read error")
		return
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if err := s.Store.EnsureBlobDir(hash); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	blob := s.Store.BlobPath(hash)
	if _, err := os.Stat(blob); errors.Is(err, os.ErrNotExist) {
		_ = tmp.Close()
		if err := os.Rename(tmp.Name(), blob); err != nil {
			// cross-device fallback
			if err := copyFile(tmp.Name(), blob); err != nil {
				writeErr(w, http.StatusInternalServerError, "store blob failed")
				return
			}
			_ = os.Remove(tmp.Name())
		}
	}

	mime := r.Header.Get("X-File-Mime")
	if mime == "" {
		mime = r.Header.Get("Content-Type")
	}
	if mime == "" || mime == "application/octet-stream" {
		mime = detectMime(filePath)
	}
	mtime := time.Now().UTC()
	if raw := r.Header.Get("X-File-Mtime"); raw != "" {
		if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			mtime = t
		}
	}
	deviceID := r.Header.Get("X-Device-Id")
	galleryKey := r.Header.Get("X-Gallery-Key")
	width, _ := strconv.Atoi(r.Header.Get("X-Width"))
	height, _ := strconv.Atoi(r.Header.Get("X-Height"))
	var takenAt *time.Time
	if raw := r.Header.Get("X-Taken-At"); raw != "" {
		if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			takenAt = &t
		}
	}

	if err := s.Store.EnsureParentDirs(c.UserID, filePath, deviceID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	meta := &store.FileMeta{
		UserID:     c.UserID,
		Path:       filePath,
		Name:       filepath.Base(filePath),
		IsDir:      false,
		Size:       n,
		Hash:       hash,
		Mime:       mime,
		MTime:      mtime,
		DeviceID:   deviceID,
		GalleryKey: galleryKey,
		Width:      width,
		Height:     height,
		TakenAt:    takenAt,
	}
	if err := s.Store.UpsertFile(meta); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	p := strings.TrimPrefix(r.URL.Path, "/api/sync/download/")
	p = strings.Trim(path.Clean("/"+p), "/")
	f, err := s.Store.GetFileByPath(c.UserID, p)
	if err != nil || f.Deleted || f.IsDir {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	serveBlob(w, r, s.Store.BlobPath(f.Hash), f)
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	// Remote open with Range support — same as download but Content-Disposition inline
	c := claimsFrom(r)
	p := strings.TrimPrefix(r.URL.Path, "/api/open/")
	p = strings.Trim(path.Clean("/"+p), "/")
	f, err := s.Store.GetFileByPath(c.UserID, p)
	if err != nil || f.Deleted || f.IsDir {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Disposition", "inline; filename=\""+f.Name+"\"")
	serveBlob(w, r, s.Store.BlobPath(f.Hash), f)
}

func serveBlob(w http.ResponseWriter, r *http.Request, blob string, f *store.FileMeta) {
	file, err := os.Open(blob)
	if err != nil {
		writeErr(w, http.StatusNotFound, "blob missing")
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "stat error")
		return
	}
	w.Header().Set("Content-Type", f.Mime)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("X-Content-Hash", f.Hash)
	w.Header().Set("ETag", `"`+f.Hash+`"`)
	http.ServeContent(w, r, f.Name, f.MTime, file)
	_ = stat
}

func (s *Server) handleGallery(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, err := s.Store.ListGallery(c.UserID, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []store.FileMeta{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGallerySync(w http.ResponseWriter, r *http.Request) {
	// Same as upload but requires gallery_key — convenience endpoint for Android
	if r.Header.Get("X-Gallery-Key") == "" {
		writeErr(w, http.StatusBadRequest, "X-Gallery-Key required")
		return
	}
	if r.Header.Get("X-File-Path") == "" {
		key := r.Header.Get("X-Gallery-Key")
		name := filepath.Base(r.URL.Query().Get("name"))
		if name == "" || name == "." {
			name = key + ".jpg"
		}
		r.Header.Set("X-File-Path", "Gallery/"+name)
	}
	s.handleUpload(w, r)
}

func (s *Server) handleTrash(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	switch r.Method {
	case http.MethodGet:
		items, err := s.Store.ListTrash(c.UserID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if items == nil {
			items = []store.FileMeta{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodDelete:
		// Empty trash
		n, err := s.Store.EmptyTrash(c.UserID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"purged": n})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleTrashItem(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	rest := strings.TrimPrefix(r.URL.Path, "/api/trash/")
	action, p, ok := strings.Cut(rest, "/")
	if !ok {
		// /api/trash/restore with body, or path only for purge
		action = rest
		p = ""
	}
	p = strings.Trim(path.Clean("/"+p), "/")

	switch r.Method {
	case http.MethodPost:
		if action != "restore" {
			writeErr(w, http.StatusBadRequest, "use /api/trash/restore/<path>")
			return
		}
		if p == "" || p == "." {
			var body struct {
				Path string `json:"path"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			p = strings.Trim(body.Path, "/")
		}
		if p == "" {
			writeErr(w, http.StatusBadRequest, "path required")
			return
		}
		f, err := s.Store.Restore(c.UserID, p)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, f)
	case http.MethodDelete:
		// Permanent delete: /api/trash/purge/<path> or /api/trash/<path>
		if action == "purge" {
			// p already set from Cut
		} else if action != "" && p == "" {
			p = action
		}
		if p == "" || p == "." {
			writeErr(w, http.StatusBadRequest, "path required")
			return
		}
		if err := s.Store.Purge(c.UserID, p); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"purged": true, "path": p})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func detectMime(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".pdf":
		return "application/pdf"
	case ".txt", ".md", ".log":
		return "text/plain; charset=utf-8"
	case ".json":
		return "application/json"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
