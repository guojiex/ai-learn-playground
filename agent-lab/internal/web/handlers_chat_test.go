package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
)

// fakeStreamClient 用 channel 注入 chunk, 满足 llm.Client.
type fakeStreamClient struct {
	chunks []llm.StreamChunk
}

func (f *fakeStreamClient) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, nil
}

func (f *fakeStreamClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	out := make(chan llm.StreamChunk, len(f.chunks)+1)
	go func() {
		defer close(out)
		for _, c := range f.chunks {
			select {
			case <-ctx.Done():
				out <- llm.StreamChunk{Err: ctx.Err()}
				return
			default:
			}
			out <- c
		}
	}()
	return out, nil
}

func newTestServer(t *testing.T, client llm.Client) *Server {
	t.Helper()
	cfg := config.Config{
		Profile:        "L",
		BaseURL:        "http://127.0.0.1:8080/v1",
		APIKey:         "sk-local",
		ModelChat:      "qwen2.5-7b-instruct",
		RequestTimeout: 10 * time.Second,
		MaxRetries:     1,
	}
	srv, err := NewServer(cfg, client)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv
}

func TestChatPage_Renders(t *testing.T) {
	srv := newTestServer(t, &fakeStreamClient{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"agent-lab", "Chat", "qwen2.5-7b-instruct", "/static/chat.js"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q\n%s", want, body)
		}
	}
}

func TestChatAPI_StreamsSSE(t *testing.T) {
	usage := &llm.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8}
	client := &fakeStreamClient{
		chunks: []llm.StreamChunk{
			{DeltaContent: "he"},
			{DeltaContent: "llo"},
			{FinishReason: "stop", Usage: usage},
		},
	}
	srv := newTestServer(t, client)

	body, _ := json.Marshal(map[string]any{
		"system":  "be brief",
		"message": "hi",
	})
	rr := newFlushRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rr.Code, rr.Body.String())
	}
	got := rr.Body.String()
	for _, want := range []string{
		"event: start",
		"event: delta",
		`"content":"he"`,
		`"content":"llo"`,
		"event: finish",
		"event: usage",
		"event: done",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("SSE missing %q\nfull body:\n%s", want, got)
		}
	}
}

func TestChatAPI_BadInput(t *testing.T) {
	srv := newTestServer(t, &fakeStreamClient{})
	rr := newFlushRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":""}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestStaticAssetsServed(t *testing.T) {
	srv := newTestServer(t, &fakeStreamClient{})
	for _, path := range []string{"/static/style.css", "/static/chat.js"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s -> %d", path, rr.Code)
		}
		if rr.Body.Len() < 50 {
			t.Fatalf("%s body too short: %d", path, rr.Body.Len())
		}
	}
}

func TestRootRedirects(t *testing.T) {
	srv := newTestServer(t, &fakeStreamClient{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/chat" {
		t.Fatalf("location: %q", loc)
	}
}

func TestPlaceholderPanels(t *testing.T) {
	srv := newTestServer(t, &fakeStreamClient{})
	for _, path := range []string{"/tools", "/plan", "/multi", "/approvals", "/traces", "/router", "/settings"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		srv.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s -> %d", path, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "agent-lab") {
			t.Fatalf("%s body missing layout", path)
		}
	}
}

// Sanity: SSE 帧应以空行结束.
func TestSSEFrameSplit(t *testing.T) {
	rr := newFlushRecorder()
	writeSSE(rr, rr, "delta", map[string]any{"content": "x"})
	out := rr.Body.String()
	if !strings.HasSuffix(out, "\n\n") {
		t.Fatalf("expected SSE frame to end with blank line, got %q", out)
	}
	if !strings.HasPrefix(out, "event: delta\ndata: ") {
		t.Fatalf("unexpected SSE prefix: %q", out)
	}
}
