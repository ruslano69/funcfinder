package knowledge

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"

	sqlite "modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("vec_distance_cosine", 2, sqlVecDistanceCosine)
	sqlite.MustRegisterDeterministicScalarFunction("vec_distance_l2", 2, sqlVecDistanceL2)
}

func sqlVecDistanceCosine(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	a, b, err := blobArgs(args)
	if err != nil {
		return nil, err
	}
	return cosineDistance(a, b), nil
}

func sqlVecDistanceL2(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	a, b, err := blobArgs(args)
	if err != nil {
		return nil, err
	}
	return l2Distance(a, b), nil
}

func blobArgs(args []driver.Value) ([]float32, []float32, error) {
	ba, ok := args[0].([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("arg 0: expected BLOB, got %T", args[0])
	}
	bb, ok := args[1].([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("arg 1: expected BLOB, got %T", args[1])
	}
	return blobToFloat32Slice(ba), blobToFloat32Slice(bb), nil
}

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
// Returns 1.0 for zero vectors or length mismatches.
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

// l2Distance returns the Euclidean distance between a and b.
// Returns +Inf for zero-length or mismatched vectors.
func l2Distance(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return math.Inf(1)
	}
	var sum float64
	for i := range a {
		d := float64(a[i]) - float64(b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}
