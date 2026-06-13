package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChat_NonStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("missing auth header: %q", got)
		}
		var got ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		if got.Stream {
			t.Fatalf("expected non-stream request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"model":"test-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`)
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL+"/v1", "test-key", 5*time.Second)
	resp, err := c.Chat(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("chat err: %v", err)
	}
	if resp.Message.Content != "hello" {
		t.Fatalf("unexpected content: %q", resp.Message.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}

func TestChat_RetriesOn5xx(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{}}`)
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL+"/v1", "k", 5*time.Second, WithMaxRetries(3))
	resp, err := c.Chat(context.Background(), ChatRequest{
		Model:    "m",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("chat err: %v", err)
	}
	if resp.Message.Content != "ok" {
		t.Fatalf("unexpected content: %q", resp.Message.Content)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestChat_NoRetryOn4xx(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, `{"error":"bad"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL+"/v1", "k", 5*time.Second, WithMaxRetries(3))
	_, err := c.Chat(context.Background(), ChatRequest{
		Model:    "m",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if apiErr, ok := err.(*APIError); !ok {
		t.Fatalf("expected *APIError, got %T", err)
	} else if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", apiErr.StatusCode)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter not Flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		write := func(payload string) {
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
		write(`{"choices":[{"index":0,"delta":{"role":"assistant","content":"he"},"finish_reason":""}]}`)
		write(`{"choices":[{"index":0,"delta":{"content":"llo"},"finish_reason":""}]}`)
		write(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
		write("[DONE]")
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL+"/v1", "k", 5*time.Second)
	ch, err := c.ChatStream(context.Background(), ChatRequest{
		Model:    "m",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream err: %v", err)
	}
	var got strings.Builder
	var finish string
	var usage *Usage
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("chunk err: %v", chunk.Err)
		}
		got.WriteString(chunk.DeltaContent)
		if chunk.FinishReason != "" {
			finish = chunk.FinishReason
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}
	if got.String() != "hello" {
		t.Fatalf("unexpected content: %q", got.String())
	}
	if finish != "stop" {
		t.Fatalf("unexpected finish: %q", finish)
	}
	if usage == nil || usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestChatStream_CtxCancel(t *testing.T) {
	// server 永远不结束流, 模拟长生成.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"slow\"},\"finish_reason\":\"\"}]}\n\n")
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	c := NewOpenAIClient(srv.URL+"/v1", "k", 5*time.Second)
	ch, err := c.ChatStream(ctx, ChatRequest{
		Model:    "m",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream err: %v", err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("stream did not close after ctx cancel")
		case _, ok := <-ch:
			if !ok {
				return
			}
		}
	}
}
