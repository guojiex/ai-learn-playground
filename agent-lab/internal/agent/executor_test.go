package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// fakePlanClient 满足 llm.Client, 返回预设的回复.
type fakePlanClient struct {
	chatResp  string
	chatErr   error
	callCount int
}

func (f *fakePlanClient) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	f.callCount++
	if f.chatErr != nil {
		return llm.ChatResponse{}, f.chatErr
	}
	return llm.ChatResponse{
		Message:      llm.Message{Role: llm.RoleAssistant, Content: f.chatResp},
		FinishReason: "stop",
		Usage:        llm.Usage{TotalTokens: 10},
	}, nil
}

func (f *fakePlanClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func TestPlanner_Plan(t *testing.T) {
	planJSON := `{"goal":"g","tasks":[{"id":"t1","name":"search","tool":"kb_search","args":{"query":"test"}},{"id":"t2","name":"write","depends":["t1"],"agent":"writer","prompt":"write about {t1.output}"}]}`
	client := &fakePlanClient{chatResp: planJSON}
	reg := tools.NewRegistry()
	p := NewPlanner(client, reg, "test-model")
	plan, err := p.Plan(context.Background(), "g")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(plan.Tasks))
	}
	if plan.Tasks[1].Prompt != "write about {t1.output}" {
		t.Fatalf("prompt=%s", plan.Tasks[1].Prompt)
	}
}

func TestPlanner_PlanRetryOnParseFail(t *testing.T) {
	client := &fakePlanClient{chatResp: "not json at all"}
	reg := tools.NewRegistry()
	p := NewPlanner(client, reg, "test-model")
	_, err := p.Plan(context.Background(), "g")
	if err == nil {
		t.Fatal("expected error after all retries fail")
	}
	if client.callCount != 3 {
		t.Fatalf("expected 3 attempts (1 + 2 retries), got %d", client.callCount)
	}
}

func TestExecutor_ExecuteAllOK(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&planEchoTool{name: "kb_search"})
	client := &fakePlanClient{chatResp: "这是文案内容"}
	planner := &Planner{client: client, reg: reg, model: "m", maxRetry: 0}
	executor := NewExecutor(planner, client, reg, "m")

	plan := &Plan{
		Goal: "test",
		Tasks: []Task{
			{ID: "t1", Name: "search", Tool: "kb_search", Args: json.RawMessage(`{"query":"毛巾"}`)},
			{ID: "t2", Name: "write", Depends: []string{"t1"}, Agent: "writer", Prompt: "根据 {t1.output} 写文案"},
		},
	}
	run, err := executor.Execute(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != "ok" {
		t.Fatalf("status=%s, want ok", run.Status)
	}
	if len(run.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(run.Results))
	}
	for _, r := range run.Results {
		if r.Status != TaskOK {
			t.Fatalf("task %s status=%s, want ok", r.TaskID, r.Status)
		}
	}
}

func TestExecutor_FailureSkipsDependents(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&planFailTool{name: "bad_tool"})
	reg.Register(&planEchoTool{name: "good_tool"})
	client := &fakePlanClient{chatResp: "ok"}
	planner := &Planner{client: client, reg: reg, model: "m", maxRetry: 0}
	executor := NewExecutor(planner, client, reg, "m")
	executor.maxReplan = 0

	plan := &Plan{
		Goal: "test",
		Tasks: []Task{
			{ID: "t1", Name: "fail", Tool: "bad_tool"},
			{ID: "t2", Name: "after", Depends: []string{"t1"}, Tool: "good_tool"},
		},
	}
	run, err := executor.Execute(context.Background(), plan, nil)
	if err == nil {
		t.Fatal("expected error for failed task")
	}
	if run.Status != "fail" {
		t.Fatalf("status=%s, want fail", run.Status)
	}
	foundFail := false
	foundSkip := false
	for _, r := range run.Results {
		if r.TaskID == "t1" && r.Status == TaskFail {
			foundFail = true
		}
		if r.TaskID == "t2" && r.Status == TaskSkipped {
			foundSkip = true
		}
	}
	if !foundFail {
		t.Fatal("t1 should be fail")
	}
	if !foundSkip {
		t.Fatal("t2 should be skipped")
	}
}

func TestExecutor_ParallelExecution(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(&planSlowTool{name: "slow_a", delay: 50 * time.Millisecond})
	reg.Register(&planSlowTool{name: "slow_b", delay: 50 * time.Millisecond})
	client := &fakePlanClient{chatResp: "ok"}
	planner := &Planner{client: client, reg: reg, model: "m", maxRetry: 0}
	executor := NewExecutor(planner, client, reg, "m")

	plan := &Plan{
		Goal: "test",
		Tasks: []Task{
			{ID: "t1", Tool: "slow_a"},
			{ID: "t2", Tool: "slow_b"},
			{ID: "t3", Agent: "writer", Depends: []string{"t1", "t2"}, Prompt: "combine"},
		},
	}
	start := time.Now()
	run, err := executor.Execute(context.Background(), plan, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != "ok" {
		t.Fatalf("status=%s", run.Status)
	}
	if elapsed > 90*time.Millisecond {
		t.Fatalf("parallel exec too slow: %v (expected <90ms for 2x50ms parallel)", elapsed)
	}
}

func TestSubstituteRefsStr(t *testing.T) {
	out := substituteRefsStr("hello {t1.output} world", map[string]string{"t1": "result"})
	if out != "hello result world" {
		t.Fatalf("got %q", out)
	}
}

// planEchoTool 是一个返回 args 的测试工具.
type planEchoTool struct{ name string }

func (t *planEchoTool) Schema() llm.ToolSchema {
	return tools.Schema(t.name, "echo", map[string]any{"type": "object"})
}
func (t *planEchoTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	return string(args), nil
}

// planFailTool 总是返回错误.
type planFailTool struct{ name string }

func (t *planFailTool) Schema() llm.ToolSchema {
	return tools.Schema(t.name, "fail", map[string]any{"type": "object"})
}
func (t *planFailTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	return "", errors.New("intentional failure")
}

// planSlowTool 睡一会儿再返回.
type planSlowTool struct {
	name  string
	delay time.Duration
}

func (t *planSlowTool) Schema() llm.ToolSchema {
	return tools.Schema(t.name, "slow", map[string]any{"type": "object"})
}
func (t *planSlowTool) Invoke(ctx context.Context, args json.RawMessage) (string, error) {
	time.Sleep(t.delay)
	return "done", nil
}
