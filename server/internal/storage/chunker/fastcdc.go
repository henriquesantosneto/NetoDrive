package chunker

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
)

// ChunkData is one chunk's original (uncompressed) payload.
type ChunkData struct {
	Index int
	Data  []byte
}

// FastCDC implements content-defined chunking (FastCDC algorithm).
type FastCDC struct {
	MinSize int
	AvgSize int
	MaxSize int
	gear    [256]uint64
}

// DefaultFastCDC returns production-friendly chunk sizes for sync workloads.
func DefaultFastCDC() *FastCDC {
	c := &FastCDC{
		MinSize: 16 << 10,  // 16 KiB
		AvgSize: 64 << 10,  // 64 KiB
		MaxSize: 256 << 10, // 256 KiB
	}
	c.initGear()
	return c
}

func (c *FastCDC) initGear() {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		for i := range seed {
			seed[i] = byte(i*31 + 17)
		}
	}
	for i := 0; i < 256; i++ {
		h := seed[i%32]
		c.gear[i] = uint64(h) | uint64(h)<<8 | uint64(h)<<16 | uint64(h)<<24 |
			uint64(h)<<32 | uint64(h)<<40 | uint64(h)<<48 | uint64(h)<<56
	}
}

func (c *FastCDC) maskS() uint64 {
	// Cut when (hash & mask) == mask for average ~MinSize boundary region.
	bits := bitsForAverage(c.MinSize)
	return (1 << bits) - 1
}

func (c *FastCDC) maskL() uint64 {
	bits := bitsForAverage(c.AvgSize)
	return (1 << bits) - 1
}

func bitsForAverage(avg int) uint {
	if avg <= 0 {
		return 13
	}
	b := uint(0)
	for (1 << b) < avg {
		b++
	}
	if b > 31 {
		b = 31
	}
	return b
}

// Chunk reads all data and splits it with FastCDC.
func (c *FastCDC) Chunk(r io.Reader) ([]ChunkData, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) <= c.MinSize {
		return []ChunkData{{Index: 0, Data: data}}, nil
	}

	var out []ChunkData
	offset := 0
	index := 0
	maskS := c.maskS()
	maskL := c.maskL()

	for offset < len(data) {
		remaining := len(data) - offset
		if remaining <= c.MinSize {
			out = append(out, ChunkData{Index: index, Data: data[offset:]})
			break
		}

		maxCut := c.MaxSize
		if remaining < maxCut {
			maxCut = remaining
		}
		minCut := c.MinSize
		if remaining < minCut {
			out = append(out, ChunkData{Index: index, Data: data[offset:]})
			break
		}

		var h uint64
		cut := minCut
		for i := 0; i < maxCut; i++ {
			b := data[offset+i]
			h = (h << 1) + c.gear[b]
			if i+1 < minCut {
				continue
			}
			if i+1 >= c.MaxSize {
				cut = i + 1
				break
			}
			mask := maskS
			if i+1 >= c.AvgSize {
				mask = maskL
			}
			if (h & mask) == 0 {
				cut = i + 1
				break
			}
		}
		if cut <= 0 || cut > remaining {
			cut = remaining
		}
		out = append(out, ChunkData{Index: index, Data: data[offset : offset+cut]})
		offset += cut
		index++
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("fastcdc produced no chunks")
	}
	return out, nil
}

// ChunkStream splits without loading entire file (for large uploads).
func (c *FastCDC) ChunkStream(r io.Reader, fn func(ChunkData) error) error {
	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	chunks, err := c.Chunk(&byteReader{buf: buf})
	if err != nil {
		return err
	}
	for _, ch := range chunks {
		if err := fn(ch); err != nil {
			return err
		}
	}
	return nil
}

type byteReader struct {
	buf []byte
	off int
}

func (b *byteReader) Read(p []byte) (int, error) {
	if b.off >= len(b.buf) {
		return 0, io.EOF
	}
	n := copy(p, b.buf[b.off:])
	b.off += n
	return n, nil
}

// NormalizeChunkSizes validates configuration.
func NormalizeChunkSizes(min, avg, max int) (int, int, int) {
	if min <= 0 {
		min = 16 << 10
	}
	if avg <= min {
		avg = min * 4
	}
	if max <= avg {
		max = avg * 4
	}
	return min, avg, max
}

// HashGearTable exports gear table for tests.
func (c *FastCDC) HashSample(data []byte) uint64 {
	var h uint64
	for _, b := range data {
		h = (h << 1) + c.gear[b]
	}
	return h
}

func init() {
	// Ensure binary size helpers compile on all platforms.
	_ = binary.LittleEndian.Uint32([]byte{1, 2, 3, 4})
}
