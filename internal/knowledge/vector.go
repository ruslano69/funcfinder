package knowledge

import (
	"encoding/binary"
	"math"
)

func float32SliceToBlob(v []float32) []byte {
	b := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

func blobToFloat32Slice(b []byte) []float32 {
	n := len(b) / 4
	v := make([]float32, n)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

// cosineDistance returns 1 - cosine_similarity(a, b).
// Returns 1.0 (maximum distance) for zero vectors or length mismatches.
func cosineDistance(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 1.0
	}
	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	return 1.0 - dot/(math.Sqrt(normA)*math.Sqrt(normB))
}
