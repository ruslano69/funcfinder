package internal

import (
	"path/filepath"
	"strings"
)

// ShardKeyForPath computes the shard key for a file path relative to rootDir.
// splitBy "file" → use full relative path; "dir" → use parent directory.
func ShardKeyForPath(absPath, rootDir, splitBy string) string {
	relPath, err := filepath.Rel(rootDir, absPath)
	if err != nil {
		relPath = absPath
	}
	if splitBy == "file" {
		return relPath
	}
	key := filepath.Dir(relPath)
	if key == "." {
		key = ""
	}
	return key
}

// PathToShardName converts a shard key to a flat JSON filename.
// e.g. "internal/auth" → "internal_auth.json", "" → "root.json"
func PathToShardName(key string) string {
	normalized := strings.ReplaceAll(key, string(filepath.Separator), "_")
	normalized = strings.ReplaceAll(normalized, "/", "_")
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.TrimLeft(normalized, "_")
	if normalized == "" {
		normalized = "root"
	}
	return normalized + ".json"
}
