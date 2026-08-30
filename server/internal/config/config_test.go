package config_test

import (
	"os"
	"testing"

	"github.com/netodrive/server/internal/config"
)

func TestChunkStorageEnabledByDefault(t *testing.T) {
	t.Setenv("NETODRIVE_CHUNK_STORAGE", "")
	cfg := config.Load()
	if !cfg.ChunkStorage {
		t.Fatal("expected chunk storage enabled by default")
	}
}

func TestChunkStorageCanBeDisabled(t *testing.T) {
	t.Setenv("NETODRIVE_CHUNK_STORAGE", "0")
	cfg := config.Load()
	if cfg.ChunkStorage {
		t.Fatal("expected chunk storage disabled with NETODRIVE_CHUNK_STORAGE=0")
	}
}

func TestChunkStorageExplicitEnable(t *testing.T) {
	t.Setenv("NETODRIVE_CHUNK_STORAGE", "1")
	cfg := config.Load()
	if !cfg.ChunkStorage {
		t.Fatal("expected chunk storage enabled")
	}
}

func TestChunkStorageEnvIsolation(t *testing.T) {
	// Ensure tests don't leak env to other packages in parallel runs.
	_ = os.Unsetenv("NETODRIVE_CHUNK_STORAGE")
}
