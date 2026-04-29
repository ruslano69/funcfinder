//go:build !xxh3

package internal

import (
	"encoding/binary"
	"hash/fnv"
	"io"
	"os"
	"sort"
)

// computeFileChecksum computes FNV-1a 128-bit checksum of a file.
// Fallback when xxh3 build tag is not set.
func computeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := fnv.New128a()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hexUint64Pair(h.Sum(nil)), nil
}

// computeShardChecksum computes combined checksum for a list of file paths.
func computeShardChecksum(paths []string) string {
	sort.Strings(paths)
	h := fnv.New128a()
	for _, p := range paths {
		checksum, err := computeFileChecksum(p)
		if err != nil {
			h.Write([]byte(p))
			continue
		}
		h.Write([]byte(p + ":" + checksum + "\n"))
	}
	return hexUint64Pair(h.Sum(nil))
}

// hexUint64Pair encodes a 16-byte slice as hex string.
func hexUint64Pair(b []byte) string {
	hi := binary.BigEndian.Uint64(b[:8])
	lo := binary.BigEndian.Uint64(b[8:])
	return uint64Hex(hi) + uint64Hex(lo)
}

const hexChars = "0123456789abcdef"

func uint64Hex(v uint64) string {
	buf := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		buf[i] = hexChars[v&0xf]
		v >>= 4
	}
	return string(buf)
}
