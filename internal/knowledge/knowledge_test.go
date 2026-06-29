package knowledge

import (
	"database/sql"
	"math"
	"path/filepath"
	"testing"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestAddAndCount(t *testing.T) {
	db := openDB(t)

	n, err := Count(db)
	if err != nil || n != 0 {
		t.Fatalf("want count=0, got %d err=%v", n, err)
	}
	id, err := Add(db, "title1", "content one two three", "general", "{}", nil)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id != 1 {
		t.Fatalf("want id=1, got %d", id)
	}
	n, _ = Count(db)
	if n != 1 {
		t.Fatalf("want count=1, got %d", n)
	}
}

func TestFTSSearch(t *testing.T) {
	db := openDB(t)
	Add(db, "storage docs", "how to store candidate data in sqlite", "general", "{}", nil)
	Add(db, "network errors", "connection refused check the port and host", "error", "{}", nil)

	results, err := SearchFTS(db, "candidate", 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(results) != 1 || results[0].Title != "storage docs" {
		t.Fatalf("unexpected results: %v", results)
	}
}

func TestVectorSearchCosine(t *testing.T) {
	db := openDB(t)
	Add(db, "doc A", "content A", "general", "{}", []float32{1, 0, 0})
	Add(db, "doc B", "content B", "general", "{}", []float32{0, 1, 0})
	Add(db, "no vec", "content C", "general", "{}", nil)

	results, err := SearchVec(db, []float32{0.9, 0.1, 0}, 2, MetricCosine, "")
	if err != nil {
		t.Fatalf("SearchVec: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results (only docs with embeddings), got %d", len(results))
	}
	if results[0].Title != "doc A" {
		t.Fatalf("closest should be doc A, got %q", results[0].Title)
	}
}

func TestVectorSearchL2(t *testing.T) {
	db := openDB(t)
	Add(db, "doc A", "content A", "general", "{}", []float32{1, 0})
	Add(db, "doc B", "content B", "general", "{}", []float32{0, 1})

	results, err := SearchVec(db, []float32{0.9, 0.1}, 1, MetricL2, "")
	if err != nil {
		t.Fatalf("SearchVec L2: %v", err)
	}
	if len(results) != 1 || results[0].Title != "doc A" {
		t.Fatalf("L2 closest should be doc A, got %v", results)
	}
}

func TestVectorSearchFilterType(t *testing.T) {
	db := openDB(t)
	Add(db, "error doc", "content", "error", "{}", []float32{1, 0})
	Add(db, "general doc", "content", "general", "{}", []float32{1, 0})

	results, err := SearchVec(db, []float32{1, 0}, 10, MetricCosine, "error")
	if err != nil {
		t.Fatalf("SearchVec filter: %v", err)
	}
	if len(results) != 1 || results[0].Type != "error" {
		t.Fatalf("expected 1 error doc, got %v", results)
	}
}

func TestHybridSearch(t *testing.T) {
	db := openDB(t)
	Add(db, "sqlite storage", "store data in sqlite database fts5", "general", "{}", []float32{1, 0})
	Add(db, "network fix", "fix connection refused postgres error", "error", "{}", []float32{0, 1})

	results, err := SearchHybrid(db, "sqlite database", []float32{0.9, 0.1}, 5, MetricCosine, "")
	if err != nil {
		t.Fatalf("SearchHybrid: %v", err)
	}
	if len(results) == 0 || results[0].Title != "sqlite storage" {
		t.Fatalf("expected top result 'sqlite storage', got %v", results)
	}
}

func TestCosineDistance(t *testing.T) {
	cases := []struct {
		a, b []float32
		want float64
	}{
		{[]float32{1, 0, 0}, []float32{1, 0, 0}, 0.0},
		{[]float32{1, 0}, []float32{0, 1}, 1.0},
		{[]float32{0, 0}, []float32{1, 0}, 1.0},
		{[]float32{1}, []float32{1, 0}, 1.0},
	}
	for _, c := range cases {
		got := cosineDistance(c.a, c.b)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("cosineDistance(%v,%v): want %f got %f", c.a, c.b, c.want, got)
		}
	}
}

func TestBlobRoundtrip(t *testing.T) {
	in := []float32{0.1, 0.2, -0.3, 1.5, 0}
	out := blobToFloat32Slice(float32SliceToBlob(in))
	for i := range in {
		if in[i] != out[i] {
			t.Fatalf("mismatch at %d: %f vs %f", i, in[i], out[i])
		}
	}
}

func TestDelete(t *testing.T) {
	db := openDB(t)
	id, _ := Add(db, "tmp", "tmp content", "general", "{}", nil)
	if err := Delete(db, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	n, _ := Count(db)
	if n != 0 {
		t.Fatalf("expected 0 after delete, got %d", n)
	}
}
