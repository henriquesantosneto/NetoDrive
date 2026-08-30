package compressor

import (
	"fmt"

	"github.com/klauspost/compress/zstd"
)

const algorithm = "zstd"

// Zstd compresses chunks with moderate level (speed vs ratio).
type Zstd struct {
	enc *zstd.Encoder
	dec *zstd.Decoder
}

func NewZstd() (*Zstd, error) {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, err
	}
	dec, err := zstd.NewReader(nil)
	if err != nil {
		_ = enc.Close()
		return nil, err
	}
	return &Zstd{enc: enc, dec: dec}, nil
}

func (z *Zstd) Algorithm() string { return algorithm }

func (z *Zstd) Compress(data []byte) ([]byte, error) {
	out := z.enc.EncodeAll(data, make([]byte, 0, len(data)))
	if len(out) == 0 && len(data) > 0 {
		return nil, fmt.Errorf("zstd compress produced empty output")
	}
	return out, nil
}

func (z *Zstd) Decompress(data []byte) ([]byte, error) {
	out, err := z.dec.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("zstd decompress: %w", err)
	}
	return out, nil
}

func (z *Zstd) Close() error {
	if z.enc != nil {
		_ = z.enc.Close()
	}
	if z.dec != nil {
		z.dec.Close()
	}
	return nil
}
