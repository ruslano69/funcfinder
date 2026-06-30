//go:build !xxh3

package internal

import (
	"encoding/hex"
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
	return hex.EncodeToString(h.Sum(nil)), nil
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
	return hex.EncodeToString(h.Sum(nil))
}
