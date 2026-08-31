package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// SHA256 implements content hashing on uncompressed chunk bytes.
type SHA256 struct{}

func NewSHA256() *SHA256 { return &SHA256{} }

func (h *SHA256) Sum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (h *SHA256) Verify(data []byte, expected string) error {
	got := h.Sum(data)
	if got != expected {
		return fmt.Errorf("chunk hash mismatch: expected %s got %s", expected, got)
	}
	return nil
}
