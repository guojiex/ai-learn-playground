package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient 是面向 OpenAI 兼容 server 的最小客户端实现.
//
// 故意不引第三方 SDK: 我们要的是看见每一根线.
type OpenAIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	maxRetries int
}

// OpenAIOption 用于构造 OpenAIClient 的可选参数.
type OpenAIOption func(*OpenAIClient)

// WithHTTPClient 注入自定义 http.Client (例如换连接池或注入 fake transport).
func WithHTTPClient(c *http.Client) OpenAIOption {
	return func(o *OpenAIClient) {
		if c != nil {
			o.httpClient = c
		}
	}
}

// WithMaxRetries 覆盖默认重试次数 (默认 3).
func WithMaxRetries(n int) OpenAIOption {
	return func(o *OpenAIClient) {
		if n >= 0 {
			o.maxRetries = n
		}
	}
}

// NewOpenAIClient 构造客户端. baseURL 形如 http://127.0.0.1:8080/v1.
func NewOpenAIClient(baseURL, apiKey string, timeout time.Duration, opts ...OpenAIOption) *OpenAIClient {
	c := &OpenAIClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
		maxRetries: 3,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// APIError 是非 2xx 响应的结构化错误, 调用方可类型断言以决定是否重试.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openai api error: status=%d body=%s", e.StatusCode, truncate(e.Body, 512))
}

// Chat 实现 Client.
func (c *OpenAIClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal request: %w", err)
	}
	respBody, err := c.doWithRetry(ctx, "POST", c.baseURL+"/chat/completions", body, false)
	if err != nil {
		return ChatResponse{}, err
	}
	defer respBody.Close()
	raw, err := io.ReadAll(respBody)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("read response: %w", err)
	}
	return parseChatResponse(raw)
}

// ChatStream 实现 Client.
func (c *OpenAIClient) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	rc, err := c.doWithRetry(ctx, "POST", c.baseURL+"/chat/completions", body, true)
	if err != nil {
		return nil, err
	}
	out := make(chan StreamChunk, 16)
	go streamSSE(ctx, rc, out)
	return out, nil
}

// doWithRetry 发起 HTTP 请求并按需做指数退避重试.
// stream=true 时不读取 body, 由调用方负责关闭.
func (c *OpenAIClient) doWithRetry(ctx context.Context, method, url string, body []byte, stream bool) (io.ReadCloser, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := backoffDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		if stream {
			req.Header.Set("Accept", "text/event-stream")
		} else {
			req.Header.Set("Accept", "application/json")
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if !shouldRetryNetwork(ctx, err) {
				return nil, err
			}
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp.Body, nil
		}
		// 非 2xx: 读取并按状态码决定是否重试.
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		apiErr := &APIError{StatusCode: resp.StatusCode, Body: string(raw)}
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			lastErr = apiErr
			continue
		}
		return nil, apiErr
	}
	if lastErr == nil {
		lastErr = errors.New("retry exhausted with no error")
	}
	return nil, lastErr
}

func shouldRetryNetwork(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	// 简单粗暴: 任何非 ctx 取消的网络层错误都重试一次.
	return err != nil
}

func backoffDelay(attempt int) time.Duration {
	base := time.Duration(200*(1<<attempt)) * time.Millisecond
	if base > 5*time.Second {
		base = 5 * time.Second
	}
	// 加 0~30% 抖动避免共振.
	jitter := time.Duration(rand.Int64N(int64(base) / 3))
	return base + jitter
}

// --- 解析非流式响应 ---

type chatCompletionResp struct {
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

func parseChatResponse(raw []byte) (ChatResponse, error) {
	var r chatCompletionResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return ChatResponse{}, fmt.Errorf("decode response: %w (body=%s)", err, truncate(string(raw), 256))
	}
	if len(r.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("no choices in response: %s", truncate(string(raw), 256))
	}
	c := r.Choices[0]
	return ChatResponse{
		Message:      c.Message,
		FinishReason: c.FinishReason,
		Usage:        r.Usage,
		Model:        r.Model,
	}, nil
}

// --- 解析 SSE 流 ---

type chatCompletionChunk struct {
	Choices []struct {
		Index        int     `json:"index"`
		Delta        Message `json:"delta"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
}

func streamSSE(ctx context.Context, body io.ReadCloser, out chan<- StreamChunk) {
	defer close(out)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	// SSE 单行可能很长, 给一个足够大的 buffer.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			out <- StreamChunk{Err: ctx.Err()}
			return
		default:
		}
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return
		}
		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			out <- StreamChunk{Err: fmt.Errorf("decode chunk: %w", err)}
			return
		}
		if len(chunk.Choices) == 0 {
			if chunk.Usage != nil {
				out <- StreamChunk{Usage: chunk.Usage}
			}
			continue
		}
		ch := chunk.Choices[0]
		out <- StreamChunk{
			DeltaContent:   ch.Delta.Content,
			DeltaToolCalls: ch.Delta.ToolCalls,
			FinishReason:   ch.FinishReason,
			Usage:          chunk.Usage,
		}
	}
	if err := scanner.Err(); err != nil {
		out <- StreamChunk{Err: err}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
