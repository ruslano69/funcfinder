package knowledge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedOllama_Success(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"embedding": []float32{0.1, 0.2, 0.3},
		})
	}))
	defer srv.Close()

	emb, err := EmbedOllama(srv.URL, "nomic-embed-text", "hello world")
	if err != nil {
		t.Fatalf("EmbedOllama: %v", err)
	}
	if len(emb) != 3 {
		t.Fatalf("expected 3 dims, got %d", len(emb))
	}
	if gotBody["model"] != "nomic-embed-text" || gotBody["prompt"] != "hello world" {
		t.Fatalf("unexpected request body: %+v", gotBody)
	}
}

func TestEmbedOllama_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := EmbedOllama(srv.URL, "nomic-embed-text", "hi"); err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestEmbedOllama_EmptyEmbedding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"embedding": []float32{}})
	}))
	defer srv.Close()

	if _, err := EmbedOllama(srv.URL, "nomic-embed-text", "hi"); err == nil {
		t.Fatal("expected error for empty embedding")
	}
}

func TestEmbedOllama_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	if _, err := EmbedOllama(srv.URL, "nomic-embed-text", "hi"); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
