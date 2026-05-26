package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"hermesclaw/internal/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	JSONMode    bool      `json:"json_mode,omitempty"`
}

type ChatProvider interface {
	Chat(ctx context.Context, req ChatRequest) (string, error)
}

type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
	Dimensions() int
}

func NewChatProvider(cfg config.Config) ChatProvider {
	if cfg.DeepSeekAPIKey != "" {
		return DeepSeekChat{APIKey: cfg.DeepSeekAPIKey, BaseURL: cfg.DeepSeekBaseURL, Model: cfg.DeepSeekModel, Client: httpClient()}
	}
	return LocalChat{}
}

func NewEmbeddingProvider(cfg config.Config) EmbeddingProvider {
	if cfg.DashScopeAPIKey != "" {
		return DashScopeEmbedding{APIKey: cfg.DashScopeAPIKey, BaseURL: cfg.DashScopeBaseURL, Model: cfg.DashScopeEmbeddingModel, Client: httpClient(), Dims: 1024}
	}
	return LocalEmbedding{Dims: 1024}
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

type DeepSeekChat struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

func (p DeepSeekChat) Chat(ctx context.Context, req ChatRequest) (string, error) {
	body := map[string]any{
		"model":       p.Model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.JSONMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("deepseek status %d: %s", resp.StatusCode, string(data))
	}
	var decoded struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("deepseek returned no choices")
	}
	return strings.TrimSpace(decoded.Choices[0].Message.Content), nil
}

type DashScopeEmbedding struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
	Dims    int
}

func (p DashScopeEmbedding) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	body := map[string]any{
		"model": p.Model,
		"input": texts,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dashscope status %d: %s", resp.StatusCode, string(data))
	}
	var decoded struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	out := make([][]float64, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		out = append(out, item.Embedding)
	}
	return out, nil
}

func (p DashScopeEmbedding) Dimensions() int { return p.Dims }

type LocalChat struct{}

func (LocalChat) Chat(_ context.Context, req ChatRequest) (string, error) {
	var last string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			last = req.Messages[i].Content
			break
		}
	}
	if last == "" {
		return "已收到请求，但当前没有配置 DeepSeek API Key。", nil
	}
	return "已基于本地规则处理。当前未配置 DeepSeek API Key，因此回答会偏模板化；配置后可获得更自然的内容。\n\n" + summarize(last), nil
}

type LocalEmbedding struct {
	Dims int
}

func (p LocalEmbedding) Embed(_ context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, 0, len(texts))
	for _, text := range texts {
		out = append(out, hashEmbedding(text, p.Dims))
	}
	return out, nil
}

func (p LocalEmbedding) Dimensions() int { return p.Dims }

func hashEmbedding(text string, dims int) []float64 {
	if dims <= 0 {
		dims = 1024
	}
	vector := make([]float64, dims)
	for _, token := range tokenize(text) {
		h := fnv.New64a()
		_, _ = h.Write([]byte(token))
		value := h.Sum64()
		idx := int(value % uint64(dims))
		sign := 1.0
		if value&1 == 0 {
			sign = -1
		}
		vector[idx] += sign
	}
	seed := sha256.Sum256([]byte(text))
	for i := 0; i+8 <= len(seed); i += 8 {
		idx := int(binary.LittleEndian.Uint64(seed[i:i+8]) % uint64(dims))
		vector[idx] += 0.2
	}
	var norm float64
	for _, v := range vector {
		norm += v * v
	}
	if norm == 0 {
		return vector
	}
	norm = math.Sqrt(norm)
	for i := range vector {
		vector[i] /= norm
	}
	return vector
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '，' || r == '。' || r == '、' || r == ':' || r == '：' || r == '/' || r == '\\'
	})
	tokens := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tokens = append(tokens, part)
		runes := []rune(part)
		if len(runes) > 2 {
			for i := 0; i+2 <= len(runes); i++ {
				tokens = append(tokens, string(runes[i:i+2]))
			}
		}
	}
	return tokens
}

func summarize(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= 240 {
		return string(runes)
	}
	return string(runes[:240]) + "..."
}
