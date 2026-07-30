package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMClient talks to OpenAI-compatible endpoints (a self-hosted llama.cpp here).
// It resolves a model by role ("chat" or "embed") from config. The chat model
// backs distillation and gate-3 adjudication; the embed model backs vectoring.
type LLMClient struct {
	http   *http.Client
	byKind map[string]LLMModel
}

func newLLMClient(cfg LLMConfig, timeout time.Duration) *LLMClient {
	byKind := make(map[string]LLMModel, len(cfg.Models))
	for _, m := range cfg.Models {
		// First entry of each kind wins; later ones are ignored.
		if _, ok := byKind[m.Kind]; !ok {
			byKind[m.Kind] = m
		}
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &LLMClient{http: &http.Client{Timeout: timeout}, byKind: byKind}
}

func (c *LLMClient) model(kind string) (LLMModel, error) {
	m, ok := c.byKind[kind]
	if !ok {
		return LLMModel{}, fmt.Errorf("no %q model configured", kind)
	}
	return m, nil
}

// EmbedDim reports the configured embedding dimension.
func (c *LLMClient) EmbedDim() int { return c.byKind["embed"].Dim }

// EmbedModelName reports the configured embedding model id, used to stamp vector
// rows so a model/dim change can be detected and migrated.
func (c *LLMClient) EmbedModelName() string { return c.byKind["embed"].Model }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type apiError struct {
	Message string `json:"message"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *apiError `json:"error,omitempty"`
}

// Chat sends a system+user prompt to the chat model and returns the assistant
// text. An empty system prompt is omitted.
func (c *LLMClient) Chat(ctx context.Context, system, user string) (string, error) {
	m, err := c.model("chat")
	if err != nil {
		return "", err
	}
	var msgs []chatMessage
	if system != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: user})

	var resp chatResponse
	if err := c.post(ctx, m, "/v1/chat/completions", chatRequest{Model: m.Model, Messages: msgs}, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("chat: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("chat: empty response")
	}
	return resp.Choices[0].Message.Content, nil
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *apiError `json:"error,omitempty"`
}

// Embed returns one vector per input text. asQuery selects the asymmetric
// prefix: a query gets QueryPrefix, a stored document gets DocPrefix.
func (c *LLMClient) Embed(ctx context.Context, texts []string, asQuery bool) ([][]float32, error) {
	m, err := c.model("embed")
	if err != nil {
		return nil, err
	}
	prefix := m.DocPrefix
	if asQuery {
		prefix = m.QueryPrefix
	}
	inputs := make([]string, len(texts))
	for i, t := range texts {
		inputs[i] = prefix + t
	}

	var resp embedResponse
	if err := c.post(ctx, m, "/v1/embeddings", embedRequest{Model: m.Model, Input: inputs}, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("embed: %s", resp.Error.Message)
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("embed: got %d vectors for %d inputs", len(resp.Data), len(texts))
	}
	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

func (c *LLMClient) post(ctx context.Context, m LLMModel, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(m.Endpoint, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.APIKey)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s: %w", endpoint, err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		var sb strings.Builder
		io.CopyN(&sb, res.Body, 2048)
		return fmt.Errorf("%s: status %d: %s", endpoint, res.StatusCode, strings.TrimSpace(sb.String()))
	}
	return json.NewDecoder(res.Body).Decode(out)
}
