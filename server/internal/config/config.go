package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr           string
	DataDir        string
	DBPath         string
	JWTSecret      string
	AdminUser      string
	AdminPass      string
	MaxUploadBytes int64
	ChunkStorage   bool
}

func Load() Config {
	return Config{
		Addr:           getenv("NETODRIVE_ADDR", ":8080"),
		DataDir:        getenv("NETODRIVE_DATA", "./data"),
		DBPath:         getenv("NETODRIVE_DB", "./data/netodrive.db"),
		JWTSecret:      getenv("NETODRIVE_JWT_SECRET", "change-me-in-production-netodrive"),
		AdminUser:      getenv("NETODRIVE_ADMIN_USER", "admin"),
		AdminPass:      getenv("NETODRIVE_ADMIN_PASS", "admin123"),
		MaxUploadBytes: getenvInt64("NETODRIVE_MAX_UPLOAD", 5<<30), // 5 GiB
		ChunkStorage:   getenvBool("NETODRIVE_CHUNK_STORAGE", true),
	}
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return fallback
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}
