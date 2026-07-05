package knowledge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// EmbedOllama requests a text embedding from an Ollama-compatible
// /api/embeddings endpoint (e.g. http://localhost:11434/api/embeddings)
// using the given model, and returns the resulting float32 vector.
func EmbedOllama(url, model, prompt string) ([]float32, error) {
	body, err := json.Marshal(map[string]string{"model": model, "prompt": prompt})
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: unexpected status %d", resp.StatusCode)
	}

	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}
	if len(out.Embedding) == 0 {
		return nil, fmt.Errorf("ollama: empty embedding in response")
	}
	return out.Embedding, nil
}
