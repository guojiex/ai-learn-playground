package llm

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

// Embedder 是 embedding 后端的最小接口 (M5 起).
type Embedder interface {
	// Embed 把一批文本编码成等长向量. 返回向量的顺序与输入一致.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dim 返回向量维度, 供 VectorStore 预分配.
	Dim() int
}

// EmbedRequest 对应 OpenAI /v1/embeddings 请求体.
type EmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbedResponse 对应 OpenAI /v1/embeddings 响应体.
type EmbedResponse struct {
	Model string      `json:"model"`
	Data  []EmbedData `json:"data"`
	Usage Usage       `json:"usage"`
}

// EmbedData 是单条文本的 embedding 结果.
type EmbedData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// OpenAIEmbedder 是面向 OpenAI 兼容 server 的 embedding 客户端.
//
// 与 OpenAIClient 分开: embedding 后端可以跑在不同端口 (见 06-ui.md 的
// http://127.0.0.1:8081/v1), 且不依赖 chat 的流式逻辑.
type OpenAIEmbedder struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	dim        int
}

// NewOpenAIEmbedder 构造 embedding 客户端. baseURL 形如 http://127.0.0.1:8081/v1.
// dim 为 0 时, 第一次 Embed 成功后自动探测.
func NewOpenAIEmbedder(baseURL, apiKey, model string, timeout time.Duration, dim int) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: timeout},
		dim:        dim,
	}
}

// Embed 实现 Embedder.
func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if e == nil {
		return nil, fmt.Errorf("embedder is nil")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	req := EmbedRequest{Model: e.model, Input: texts}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embed response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	var er EmbedResponse
	if err := json.Unmarshal(raw, &er); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	out := make([][]float32, len(texts))
	for _, d := range er.Data {
		if d.Index < 0 || d.Index >= len(out) {
			continue
		}
		out[d.Index] = d.Embedding
	}
	// 探测维度.
	if e.dim == 0 && len(out) > 0 && len(out[0]) > 0 {
		e.dim = len(out[0])
	}
	return out, nil
}

// Dim 实现 Embedder.
func (e *OpenAIEmbedder) Dim() int {
	return e.dim
}

// SetDim 手动设置维度 (例如从配置读出, 避免第一次请求才探测).
func (e *OpenAIEmbedder) SetDim(d int) {
	if d > 0 {
		e.dim = d
	}
}
