package api

import (
	"context"
	"io"
	"net/http"
	"os"

	"github.com/netodrive/server/internal/store"
)

func (s *Server) AttachChunkPurgeHook() {
	if s.ChunkStorage == nil {
		return
	}
	s.Store.OnPurged = func(f *store.FileMeta) {
		if f == nil || f.StorageFileID == "" {
			return
		}
		_ = s.ChunkStorage.DeleteFile(context.Background(), f.StorageFileID)
	}
}

func (s *Server) handleStorageGC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.ChunkStorage == nil {
		writeErr(w, http.StatusBadRequest, "chunk storage disabled")
		return
	}
	if err := s.ChunkStorage.RunGC(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	stats, _ := s.ChunkStorage.Stats(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stats": stats})
}

func (s *Server) handleStorageStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.ChunkStorage == nil {
		writeErr(w, http.StatusBadRequest, "chunk storage disabled")
		return
	}
	stats, err := s.ChunkStorage.Stats(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) openStoredFile(f store.FileMeta) (io.ReadCloser, error) {
	if s.ChunkStorage != nil && f.StorageFileID != "" {
		rc, _, err := s.ChunkStorage.OpenFile(context.Background(), f.StorageFileID)
		return rc, err
	}
	return os.Open(s.Store.BlobPath(f.Hash))
}
