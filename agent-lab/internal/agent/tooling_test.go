package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// stubClient 按预设序列依次返回 ChatResponse, 用来驱动 agent loop.
type stubClient struct {
	responses []llm.ChatResponse
	calls     int32
}

func (s *stubClient) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	idx := int(atomic.AddInt32(&s.calls, 1)) - 1
	if idx >= len(s.responses) {
		return llm.ChatResponse{}, context.Canceled
	}
	return s.responses[idx], nil
}

func (s *stubClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

// echoTool 返回 args 的副本, 测试用.
type echoTool struct{ name string }

func (e echoTool) Schema() llm.ToolSchema {
	return tools.Schema(e.name, "echo for tests", map[string]any{"type": "object"})
}
func (e echoTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	return string(args), nil
}

func TestLoop_StopsOnNoToolCalls(t *testing.T) {
	client := &stubClient{
		responses: []llm.ChatResponse{
			{Message: llm.Message{Role: llm.RoleAssistant, Content: "done"}, FinishReason: "stop"},
		},
	}
	res, err := Loop(context.Background(), client, tools.NewRegistry(), []llm.Message{
		{Role: llm.RoleUser, Content: "hi"},
	}, LoopOptions{Model: "x"})
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if res.FinalMessage.Content != "done" {
		t.Fatalf("unexpected final: %q", res.FinalMessage.Content)
	}
	if res.Steps != 1 {
		t.Fatalf("expected 1 step, got %d", res.Steps)
	}
}

func TestLoop_SingleToolCall(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(echoTool{name: "echo"})

	client := &stubClient{
		responses: []llm.ChatResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Type: "function", Function: llm.FunctionCall{Name: "echo", Arguments: `{"hello":"world"}`}},
					},
				},
				FinishReason: "tool_calls",
			},
			{Message: llm.Message{Role: llm.RoleAssistant, Content: "final answer"}, FinishReason: "stop"},
		},
	}
	res, err := Loop(context.Background(), client, reg, []llm.Message{
		{Role: llm.RoleUser, Content: "use tool"},
	}, LoopOptions{Model: "x"})
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if res.FinalMessage.Content != "final answer" {
		t.Fatalf("got: %q", res.FinalMessage.Content)
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call record, got %d", len(res.ToolCalls))
	}
	rec := res.ToolCalls[0]
	if rec.Name != "echo" {
		t.Fatalf("unexpected tool name: %s", rec.Name)
	}
	if rec.Err != "" {
		t.Fatalf("unexpected err: %s", rec.Err)
	}
	if rec.Result == "" || !strings.Contains(rec.Result, "world") {
		t.Fatalf("unexpected result: %s", rec.Result)
	}
}

func TestLoop_ParallelToolCalls(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(echoTool{name: "echo"})

	client := &stubClient{
		responses: []llm.ChatResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "a", Type: "function", Function: llm.FunctionCall{Name: "echo", Arguments: `{"i":1}`}},
						{ID: "b", Type: "function", Function: llm.FunctionCall{Name: "echo", Arguments: `{"i":2}`}},
						{ID: "c", Type: "function", Function: llm.FunctionCall{Name: "echo", Arguments: `{"i":3}`}},
					},
				},
				FinishReason: "tool_calls",
			},
			{Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"}, FinishReason: "stop"},
		},
	}
	res, err := Loop(context.Background(), client, reg, []llm.Message{{Role: llm.RoleUser, Content: "go"}}, LoopOptions{Model: "x"})
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if len(res.ToolCalls) != 3 {
		t.Fatalf("expected 3 tool call records, got %d", len(res.ToolCalls))
	}
	// 顺序必须保持入参顺序
	got := []string{res.ToolCalls[0].CallID, res.ToolCalls[1].CallID, res.ToolCalls[2].CallID}
	if got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("call order broken: %v", got)
	}
}

func TestLoop_UnknownToolReturnsErrorBackToModel(t *testing.T) {
	reg := tools.NewRegistry()
	client := &stubClient{
		responses: []llm.ChatResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "x", Type: "function", Function: llm.FunctionCall{Name: "missing", Arguments: `{}`}},
					},
				},
				FinishReason: "tool_calls",
			},
			{Message: llm.Message{Role: llm.RoleAssistant, Content: "recovered"}, FinishReason: "stop"},
		},
	}
	res, err := Loop(context.Background(), client, reg, []llm.Message{{Role: llm.RoleUser, Content: "go"}}, LoopOptions{Model: "x"})
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Err == "" {
		t.Fatalf("expected error to be recorded: %+v", res.ToolCalls)
	}
	if res.FinalMessage.Content != "recovered" {
		t.Fatalf("expected loop to continue after tool error, got: %q", res.FinalMessage.Content)
	}
}

func TestLoop_MaxStepsGuard(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(echoTool{name: "echo"})
	loop := llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "loop", Type: "function", Function: llm.FunctionCall{Name: "echo", Arguments: `{}`}},
			},
		},
		FinishReason: "tool_calls",
	}
	client := &stubClient{
		responses: []llm.ChatResponse{loop, loop, loop, loop},
	}
	_, err := Loop(context.Background(), client, reg, []llm.Message{{Role: llm.RoleUser, Content: "infinite"}}, LoopOptions{Model: "x", MaxSteps: 3})
	if err == nil {
		t.Fatal("expected error on max steps")
	}
}
