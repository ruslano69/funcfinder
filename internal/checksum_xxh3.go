//go:build xxh3

package internal

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/zeebo/xxh3"
)

// computeFileChecksum computes XXH3-128 checksum of a file.
func computeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := xxh3.New128()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	s := h.Sum128()
	return fmt.Sprintf("%016x%016x", s.Hi, s.Lo), nil
}

// computeShardChecksum computes combined XXH3-128 checksum for a list of file paths.
func computeShardChecksum(paths []string) string {
	sort.Strings(paths)
	h := xxh3.New128()
	for _, p := range paths {
		checksum, err := computeFileChecksum(p)
		if err != nil {
			h.Write([]byte(p))
			continue
		}
		h.Write([]byte(p + ":" + checksum + "\n"))
	}
	s := h.Sum128()
	return fmt.Sprintf("%016x%016x", s.Hi, s.Lo)
}
