package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"ai-learn-playground/agent-lab/internal/llm"
)

// fakeChatClient 记录调用次数与最后一次请求, 满足 llm.Client.
type fakeChatClient struct {
	resp    string
	err     error
	called  int
	lastReq llm.ChatRequest
}

func (f *fakeChatClient) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	f.called++
	f.lastReq = req
	if f.err != nil {
		return llm.ChatResponse{}, f.err
	}
	return llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: f.resp}}, nil
}

func (f *fakeChatClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func TestSummarizer_CallsLLM(t *testing.T) {
	c := &fakeChatClient{resp: "摘要: 卖家要闺蜜风"}
	s := NewSummarizer(c, "test-model", 256)
	out, err := s.Summarize(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "我喜欢闺蜜风"},
		{Role: llm.RoleAssistant, Content: "好的闺蜜风"},
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if c.called != 1 {
		t.Fatalf("expected 1 LLM call, got %d", c.called)
	}
	if out != "摘要: 卖家要闺蜜风" {
		t.Fatalf("out=%q", out)
	}
	if c.lastReq.Model != "test-model" {
		t.Fatalf("model=%q", c.lastReq.Model)
	}
}

func TestSummarizer_FallbackNoClient(t *testing.T) {
	s := NewSummarizer(nil, "m", 256)
	out, err := s.Summarize(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "我喜欢闺蜜风, 多用 Emoji"},
	})
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty fallback")
	}
	if !strings.Contains(out, "闺蜜风") {
		t.Fatalf("fallback should keep keywords: %q", out)
	}
}

func TestSummarizer_PropagatesError(t *testing.T) {
	c := &fakeChatClient{err: errors.New("boom")}
	s := NewSummarizer(c, "m", 256)
	if _, err := s.Summarize(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "x"}}); err == nil {
		t.Fatal("expected error to propagate")
	}
}

// TestEnsureBudget_SummaryTriggered 验证越界时摘要被调用, 且 token 估算被填上.
func TestEnsureBudget_SummaryTriggered(t *testing.T) {
	c := &fakeChatClient{resp: "压缩后的摘要"}
	m := NewShortTerm("你是电商文案助理", 40, 10) // available=30, 强制触发摘要
	for i := 0; i < 8; i++ {
		m.Append(llm.RoleUser, fmt.Sprintf("这是第 %d 条比较长的用户消息用来撑爆预算", i))
		m.Append(llm.RoleAssistant, fmt.Sprintf("这是第 %d 条比较长的助手回复用来撑爆预算", i))
	}
	info, _ := m.EnsureBudget(context.Background(), c, "m", 256)
	if !info.DidCompress {
		t.Fatalf("expected DidCompress, got %+v", info)
	}
	if info.BeforeTokens <= 0 {
		t.Fatalf("BeforeTokens not populated: %+v", info)
	}
	if info.AfterTokens > info.BeforeTokens {
		t.Fatalf("after_tokens(%d) should be <= before_tokens(%d)", info.AfterTokens, info.BeforeTokens)
	}
	if c.called < 1 {
		t.Fatalf("summarizer LLM not called: %d", c.called)
	}
	if info.Summary != "压缩后的摘要" {
		t.Fatalf("summary=%q", info.Summary)
	}
}

// TestEnsureBudget_NoCompressUnderBudget 验证未越界时不调用 LLM.
func TestEnsureBudget_NoCompressUnderBudget(t *testing.T) {
	c := &fakeChatClient{resp: "should-not-be-used"}
	m := NewShortTerm("sys", 2048, 512)
	m.Append(llm.RoleUser, "hi")
	info, err := m.EnsureBudget(context.Background(), c, "m", 256)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if info.DidCompress {
		t.Fatalf("should not compress under budget: %+v", info)
	}
	if c.called != 0 {
		t.Fatalf("LLM should not be called: %d", c.called)
	}
}
