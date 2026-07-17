package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ruslano69/funcfinder/internal/embed"
	"github.com/ruslano69/funcfinder/internal/knowledge"
)

// fakeEmbedServer serves the Ollama /api/embed wire format, returning one
// fixed-dim vector per input so tests don't need a real model.
func fakeEmbedServer(t *testing.T, dim int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		embs := make([][]float32, len(req.Input))
		for i := range req.Input {
			v := make([]float32, dim)
			for d := range v {
				v[d] = float32(i + 1)
			}
			embs[i] = v
		}
		json.NewEncoder(w).Encode(map[string]any{"embeddings": embs})
	}))
}

func TestEmbedChunks_BatchEmbedsEach(t *testing.T) {
	ts := fakeEmbedServer(t, 4)
	defer ts.Close()

	ec := embed.New(ts.URL, "test-model")
	chunks := []knowledge.Chunk{{Content: "a"}, {Content: "b"}, {Content: "c"}}
	vecs := embedChunks(ec, chunks)
	if len(vecs) != 3 {
		t.Fatalf("want 3 vectors, got %d", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 4 {
			t.Errorf("vec[%d] dim = %d, want 4", i, len(v))
		}
	}
}

func TestEmbedChunks_DisabledClientReturnsNil(t *testing.T) {
	// No model configured → embedding disabled → nil (store BYO/FTS only).
	ec := embed.New("", "")
	if v := embedChunks(ec, []knowledge.Chunk{{Content: "x"}}); v != nil {
		t.Errorf("disabled client should embed nothing, got %v", v)
	}
}

func TestEmbedChunks_FailureDegradesToNil(t *testing.T) {
	// Dead endpoint: must warn+degrade to nil, not panic — ingestion falls
	// back to storing without vectors.
	ec := embed.New("http://127.0.0.1:1/dead", "test-model")
	if v := embedChunks(ec, []knowledge.Chunk{{Content: "x"}}); v != nil {
		t.Errorf("failed embed should degrade to nil, got %v", v)
	}
}

func TestEmbedOne(t *testing.T) {
	ts := fakeEmbedServer(t, 8)
	defer ts.Close()

	ec := embed.New(ts.URL, "test-model")
	if v := embedOne(ec, "hello"); len(v) != 8 {
		t.Fatalf("want dim 8, got %d", len(v))
	}

	// Disabled and failing clients both degrade to nil.
	if v := embedOne(embed.New("", ""), "hello"); v != nil {
		t.Errorf("disabled client should return nil, got %v", v)
	}
	if v := embedOne(embed.New("http://127.0.0.1:1/dead", "m"), "hello"); v != nil {
		t.Errorf("failed embed should return nil, got %v", v)
	}
}

func TestChunkVec_NilSafe(t *testing.T) {
	if v := chunkVec(nil, 0); v != nil {
		t.Errorf("nil batch → nil, got %v", v)
	}
	vecs := [][]float32{{1, 2}, {3, 4}}
	if v := chunkVec(vecs, 1); len(v) != 2 || v[0] != 3 {
		t.Errorf("chunkVec(vecs,1) = %v, want [3 4]", v)
	}
	if v := chunkVec(vecs, 5); v != nil {
		t.Errorf("out-of-range index → nil, got %v", v)
	}
}
