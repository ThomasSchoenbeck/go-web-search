package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	log    *log.Logger
}

func newLLMClient(cfg LLMConfig, timeout time.Duration, logger *log.Logger) *LLMClient {
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
	return &LLMClient{http: &http.Client{Timeout: timeout}, byKind: byKind, log: logger}
}

// logf logs one line per LLM call so the otherwise-silent distill/embed traffic
// to the model server is visible. It no-ops when no logger is wired.
func (c *LLMClient) logf(format string, args ...any) {
	if c.log != nil {
		c.log.Printf("     "+format, args...)
	}
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

type apiError struct {
	Message string `json:"message"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *apiError `json:"error,omitempty"`
}

// chatParams are per-call overrides of the chat model's configured behaviour,
// used by the distill preview to A/B settings without touching config. A nil
// pointer means "use the model's configured value".
type chatParams struct {
	maxTokens *int
	noThink   *bool
}

// ChatUsage reports token counts and wall time for a single chat call, so a
// caller (the distill preview) can measure cost, not just read the log.
type ChatUsage struct {
	PromptTokens     int
	CompletionTokens int
	Duration         time.Duration
}

// Chat sends a system+user prompt to the chat model and returns the assistant
// text. An empty system prompt is omitted.
func (c *LLMClient) Chat(ctx context.Context, system, user string) (string, error) {
	out, _, err := c.chat(ctx, system, user, chatParams{})
	return out, err
}

// chat is the full chat path: it applies the model's configured knobs (NoThink,
// MaxTokens, ExtraBody), lets a caller override them per call, and returns token
// usage alongside the text.
func (c *LLMClient) chat(ctx context.Context, system, user string, p chatParams) (string, ChatUsage, error) {
	m, err := c.model("chat")
	if err != nil {
		return "", ChatUsage{}, err
	}

	noThink := m.NoThink
	if p.noThink != nil {
		noThink = *p.noThink
	}
	if noThink {
		// The soft switch acts on the last turn, so append it to the user text.
		user = strings.TrimRight(user, "\n") + "\n/no_think"
	}

	var msgs []chatMessage
	if system != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: user})

	// Built as a map so ExtraBody can carry server-specific knobs (disabling a
	// thinking model, a reasoning budget, ...) without a struct field per knob.
	body := map[string]any{
		"model":    m.Model,
		"messages": msgs,
		"stream":   false,
	}
	maxTokens := m.MaxTokens
	if p.maxTokens != nil {
		maxTokens = *p.maxTokens
	}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	for k, v := range m.ExtraBody {
		body[k] = v // merged last: config wins, by design
	}

	var resp chatResponse
	start := time.Now()
	if err := c.post(ctx, m, "/chat/completions", body, &resp, ""); err != nil {
		return "", ChatUsage{}, err
	}
	usage := ChatUsage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		Duration:         time.Since(start),
	}
	if resp.Error != nil {
		return "", usage, fmt.Errorf("chat: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", usage, fmt.Errorf("chat: empty response")
	}
	return resp.Choices[0].Message.Content, usage, nil
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
// prefix: a query gets QueryPrefix, a stored document gets DocPrefix. purpose is
// a short human label (e.g. "memory fact", "search cache lookup") that, with a
// preview of the text, is logged so the embed traffic is self-explanatory.
func (c *LLMClient) Embed(ctx context.Context, texts []string, asQuery bool, purpose string) ([][]float32, error) {
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
	// Label built from the raw text (before the prefix) so the log preview is
	// the actual content, not the instruction prefix.
	if err := c.post(ctx, m, "/embeddings", embedRequest{Model: m.Model, Input: inputs}, &resp, embedLabel(purpose, asQuery, texts)); err != nil {
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

// tokenSummary reads the OpenAI-style usage block from a response body and
// renders it as ", <prompt>→<completion> tok @ <rate> tok/s". The rate exposes
// generation throughput, which is what makes a distill slow. Best-effort:
// returns "" when the server sent no usage (e.g. an error body).
func tokenSummary(raw []byte, dur time.Duration) string {
	var meta struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		return ""
	}
	u := meta.Usage
	if u.PromptTokens == 0 && u.CompletionTokens == 0 {
		return ""
	}
	rate := ""
	if u.CompletionTokens > 0 && dur > 0 {
		rate = fmt.Sprintf(" @ %.0f tok/s", float64(u.CompletionTokens)/dur.Seconds())
	}
	return fmt.Sprintf(", %d→%d tok%s", u.PromptTokens, u.CompletionTokens, rate)
}

// embedLabel builds the log annotation shown on an embed call: what it is for,
// whether it is a query or a stored document, and a short preview of the text.
func embedLabel(purpose string, asQuery bool, texts []string) string {
	kind := "doc"
	if asQuery {
		kind = "query"
	}
	preview := ""
	if len(texts) > 0 {
		preview = strings.Join(strings.Fields(texts[0]), " ")
		if r := []rune(preview); len(r) > 60 { // rune-safe truncation
			preview = string(r[:60]) + "…"
		}
	}
	extra := ""
	if len(texts) > 1 {
		extra = fmt.Sprintf(" +%d more", len(texts)-1)
	}
	return fmt.Sprintf(" [%s/%s %q%s]", purpose, kind, preview, extra)
}

func (c *LLMClient) post(ctx context.Context, m LLMModel, path string, body, out any, label string) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	// APIPath nil means "use the standard /v1"; an explicit "" means the routes
	// live at the server root (some multimodel llama-server setups).
	prefix := "/v1"
	if m.APIPath != nil {
		prefix = *m.APIPath
	}
	endpoint := strings.TrimRight(m.Endpoint, "/") + strings.TrimRight(prefix, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.APIKey)
	}
	// Log the start too: a chat/distill call can run for minutes, and without a
	// start line the log looks idle while the model works. Request size is the
	// prompt going out; response size on completion exposes a model that emits a
	// huge (e.g. thinking) reply, which is usually what makes a call slow.
	c.logf("LLM %s (%s) %s%s: %d B out, calling...", m.Name, m.Model, path, label, len(buf))
	start := time.Now()
	res, err := c.http.Do(req)
	if err != nil {
		c.logf("LLM %s (%s) %s%s: %d B request ERROR after %s: %v", m.Name, m.Model, path, label, len(buf), time.Since(start).Round(time.Millisecond), err)
		return fmt.Errorf("calling %s: %w", endpoint, err)
	}
	defer res.Body.Close()
	raw, readErr := io.ReadAll(res.Body)
	dur := time.Since(start)
	c.logf("LLM %s (%s) %s%s -> %d, %d B req / %d B resp%s in %s", m.Name, m.Model, path, label, res.StatusCode, len(buf), len(raw), tokenSummary(raw, dur), dur.Round(time.Millisecond))
	if readErr != nil {
		return fmt.Errorf("reading response from %s: %w", endpoint, readErr)
	}
	if res.StatusCode >= 300 {
		msg := raw
		if len(msg) > 2048 {
			msg = msg[:2048]
		}
		return fmt.Errorf("%s: status %d: %s", endpoint, res.StatusCode, strings.TrimSpace(string(msg)))
	}
	return json.Unmarshal(raw, out)
}
