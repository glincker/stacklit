package git

import (
	"encoding/hex"
	"sort"
)

// ComputeMerkleFromLeaves matches ComputeMerkle but takes precomputed leaf
// hashes (sha256 of path plus content), so file bodies need not stay in memory.
func ComputeMerkleFromLeaves(leaves map[string][]byte) string {
	if len(leaves) == 0 {
		return ""
	}
	paths := make([]string, 0, len(leaves))
	for p := range leaves {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	layer := make([][]byte, len(paths))
	for i, p := range paths {
		layer[i] = leaves[p]
	}
	for len(layer) > 1 {
		layer = merkleLayer(layer)
	}
	return hex.EncodeToString(layer[0])
}
