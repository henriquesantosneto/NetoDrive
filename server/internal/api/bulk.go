package api

import (
	"archive/zip"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

func (s *Server) registerBulkRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/bulk/delete", s.withAuth(s.handleBulkDelete))
	mux.HandleFunc("/api/bulk/purge", s.withAuth(s.handleBulkPurge))
	mux.HandleFunc("/api/bulk/restore", s.withAuth(s.handleBulkRestore))
	mux.HandleFunc("/api/bulk/download", s.withAuth(s.handleBulkDownload))
}

type bulkPathsBody struct {
	Paths []string `json:"paths"`
}

func readBulkPaths(r *http.Request) ([]string, error) {
	var body bulkPathsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(body.Paths))
	seen := map[string]bool{}
	for _, p := range body.Paths {
		p = strings.Trim(path.Clean("/"+p), "/")
		if p == "" || p == "." || strings.Contains(p, "..") || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out, nil
}

func (s *Server) handleBulkDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	c := claimsFrom(r)
	paths, err := readBulkPaths(r)
	if err != nil || len(paths) == 0 {
		writeErr(w, http.StatusBadRequest, "paths required")
		return
	}
	n, err := s.Store.SoftDeleteMany(c.UserID, paths)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": n})
}

func (s *Server) handleBulkPurge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	c := claimsFrom(r)
	paths, err := readBulkPaths(r)
	if err != nil || len(paths) == 0 {
		writeErr(w, http.StatusBadRequest, "paths required")
		return
	}
	n := 0
	for _, p := range paths {
		if err := s.Store.Purge(c.UserID, p); err == nil {
			n++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"purged": n})
}

func (s *Server) handleBulkRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	c := claimsFrom(r)
	paths, err := readBulkPaths(r)
	if err != nil || len(paths) == 0 {
		writeErr(w, http.StatusBadRequest, "paths required")
		return
	}
	n := 0
	for _, p := range paths {
		if _, err := s.Store.Restore(c.UserID, p); err == nil {
			n++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"restored": n})
}

func (s *Server) handleBulkDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	c := claimsFrom(r)
	paths, err := readBulkPaths(r)
	if err != nil || len(paths) == 0 {
		writeErr(w, http.StatusBadRequest, "paths required")
		return
	}

	files, err := s.Store.CollectForDownload(c.UserID, paths)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(files) == 0 {
		writeErr(w, http.StatusNotFound, "no files to download")
		return
	}

	name := "netodrive-" + time.Now().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, f := range files {
		if f.IsDir || f.Hash == "" {
			continue
		}
		src, err := s.openStoredFile(f)
		if err != nil {
			continue
		}
		h := &zip.FileHeader{
			Name:     f.Path,
			Method:   zip.Deflate,
			Modified: f.MTime,
		}
		dst, err := zw.CreateHeader(h)
		if err != nil {
			_ = src.Close()
			continue
		}
		_, _ = io.Copy(dst, src)
		_ = src.Close()
	}
}
