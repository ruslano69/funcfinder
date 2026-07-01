// Package embed is an optional, provider-agnostic embedding client for
// docsearch-server. The server stays BYO-embeddings by design (TZ §7.2) — it
// stores and compares vectors but does not compute them. This package is the
// convenience bridge: point it at any Ollama-compatible /api/embed endpoint
// (url + model) and it turns text into vectors at ingest and query time.
//
// A zero/empty Client (no model configured) is disabled: callers fall back to
// pure FTS / explicit BYO vectors, preserving the agnostic default.
package embed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DefaultURL is the standard local Ollama embeddings endpoint.
const DefaultURL = "http://localhost:11434/api/embed"

// Client calls an Ollama-compatible embeddings endpoint. The wire format is
// {"model":..,"input":[..]} -> {"embeddings":[[..]]}, which Ollama and several
// compatible servers share.
type Client struct {
	URL   string
	Model string
	HTTP  *http.Client
}

// New returns a client for the given model at url (DefaultURL if url is empty).
// An empty model yields a disabled client (Enabled reports false).
func New(url, model string) *Client {
	if url == "" {
		url = DefaultURL
	}
	return &Client{URL: url, Model: model, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// Enabled reports whether embedding is configured. When false, callers should
// skip embedding and behave as pure BYO/FTS.
func (c *Client) Enabled() bool { return c != nil && c.Model != "" }

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed returns the vector for a single text.
func (c *Client) Embed(text string) ([]float32, error) {
	vs, err := c.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return vs[0], nil
}

// EmbedBatch returns one vector per input, in order. Ollama accepts an array
// input and returns a parallel array of embeddings.
func (c *Client) EmbedBatch(texts []string) ([][]float32, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("embedding disabled (no model configured)")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embedRequest{Model: c.Model, Input: texts})
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Post(c.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed endpoint returned %s", resp.Status)
	}
	var er embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if len(er.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(er.Embeddings))
	}
	return er.Embeddings, nil
}
