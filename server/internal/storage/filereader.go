package storage

import (
	"context"
	"fmt"
	"io"
)

type fileReader struct {
	s        *Service
	ctx      context.Context
	manifest FileManifest
	pos      int64
	idx      int
	chunkBuf []byte
	chunkOff int
}

// OpenFile returns a seekable reader over a stored file manifest.
func (s *Service) OpenFile(ctx context.Context, fileID string) (io.ReadSeekCloser, int64, error) {
	man, err := s.GetManifest(ctx, fileID)
	if err != nil {
		return nil, 0, err
	}
	return &fileReader{s: s, ctx: ctx, manifest: man}, man.Size, nil
}

func (fr *fileReader) Read(p []byte) (int, error) {
	if fr.pos >= fr.manifest.Size {
		return 0, io.EOF
	}
	nTotal := 0
	for nTotal < len(p) && fr.pos < fr.manifest.Size {
		if err := fr.ensureChunkLoaded(); err != nil {
			return nTotal, err
		}
		n := copy(p[nTotal:], fr.chunkBuf[fr.chunkOff:])
		fr.chunkOff += n
		fr.pos += int64(n)
		nTotal += n
		if fr.chunkOff >= len(fr.chunkBuf) {
			fr.idx++
			fr.chunkBuf = nil
			fr.chunkOff = 0
		}
	}
	if nTotal == 0 && fr.pos >= fr.manifest.Size {
		return 0, io.EOF
	}
	return nTotal, nil
}

func (fr *fileReader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = fr.pos + offset
	case io.SeekEnd:
		abs = fr.manifest.Size + offset
	default:
		return fr.pos, fmt.Errorf("invalid whence")
	}
	if abs < 0 || abs > fr.manifest.Size {
		return fr.pos, fmt.Errorf("seek out of range")
	}
	fr.pos = abs
	fr.idx = 0
	fr.chunkBuf = nil
	fr.chunkOff = 0
	var skip int64
	for i, ch := range fr.manifest.Chunks {
		if skip+ch.Size > abs {
			fr.idx = i
			fr.chunkOff = int(abs - skip)
			return fr.pos, nil
		}
		skip += ch.Size
	}
	return fr.pos, nil
}

func (fr *fileReader) Close() error {
	fr.chunkBuf = nil
	return nil
}

func (fr *fileReader) ensureChunkLoaded() error {
	if fr.idx >= len(fr.manifest.Chunks) {
		return io.EOF
	}
	if fr.chunkBuf != nil && fr.chunkOff < len(fr.chunkBuf) {
		return nil
	}
	ref := fr.manifest.Chunks[fr.idx]
	rc, err := fr.s.GetChunk(fr.ctx, ref.Hash)
	if err != nil {
		return err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	if int64(len(data)) != ref.Size {
		return fmt.Errorf("chunk %s size mismatch", ref.Hash)
	}
	fr.chunkBuf = data
	return nil
}
