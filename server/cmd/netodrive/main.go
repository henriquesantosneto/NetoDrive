package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/netodrive/server/internal/api"
	"github.com/netodrive/server/internal/auth"
	"github.com/netodrive/server/internal/config"
	"github.com/netodrive/server/internal/storage"
	"github.com/netodrive/server/internal/store"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	st, err := store.Open(cfg.DBPath, cfg.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	authSvc := auth.New(cfg.JWTSecret)
	if err := ensureAdmin(st, cfg); err != nil {
		log.Fatal(err)
	}

	srv := api.New(cfg, st, authSvc)
	if cfg.ChunkStorage {
		root := filepath.Join(cfg.DataDir, "chunk-storage")
		chunkSvc, err := storage.Open(storage.Config{
			RootDir: root,
			DBPath:  filepath.Join(root, "storage.db"),
		})
		if err != nil {
			log.Fatal(err)
		}
		defer chunkSvc.Close()
		srv.ChunkStorage = chunkSvc
		srv.AttachChunkPurgeHook()
		log.Printf("chunk storage enabled at %s", root)
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       0, // large uploads
		WriteTimeout:      0,
	}

	log.Printf("NetoDrive listening on %s (data=%s)", cfg.Addr, cfg.DataDir)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func ensureAdmin(st *store.Store, cfg config.Config) error {
	if _, err := st.GetUserByUsername(cfg.AdminUser); err == nil {
		return nil
	}
	hash, err := auth.HashPassword(cfg.AdminPass)
	if err != nil {
		return err
	}
	_, err = st.CreateUser(cfg.AdminUser, hash)
	if err == nil {
		log.Printf("created admin user %q", cfg.AdminUser)
	}
	return err
}
