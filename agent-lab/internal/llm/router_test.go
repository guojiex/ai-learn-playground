package llm

import (
	"context"
	"errors"
	"testing"
)

func TestRegistry_ByTag(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	m, ok := reg.ByTag("fast")
	if !ok {
		t.Fatal("expected to find model with tag 'fast'")
	}
	if m.Name != "qwen2.5-3b-instruct" {
		t.Fatalf("name=%s, want qwen2.5-3b-instruct", m.Name)
	}

	m2, ok := reg.ByTag("reason")
	if !ok {
		t.Fatal("expected to find model with tag 'reason'")
	}
	if m2.Name != "qwen2.5-14b-instruct" {
		t.Fatalf("name=%s", m2.Name)
	}

	_, ok = reg.ByTag("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent tag")
	}
}

func TestRegistry_ByName(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	m, ok := reg.ByName("qwen2.5-7b-instruct")
	if !ok {
		t.Fatal("expected to find model by name")
	}
	if !contains(m.Tags, "default") {
		t.Fatalf("expected tag 'default', got %v", m.Tags)
	}
}

func TestPolicy_Evaluate_TaskTitle(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	policy := DefaultPolicy()
	result, err := policy.Evaluate("title", 100, reg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Primary.Name != "qwen2.5-3b-instruct" {
		t.Fatalf("expected 3b for title, got %s", result.Primary.Name)
	}
	if len(result.Fallbacks) != 1 {
		t.Fatalf("expected 1 fallback, got %d", len(result.Fallbacks))
	}
	if result.Fallbacks[0].Name != "qwen2.5-7b-instruct" {
		t.Fatalf("expected 7b fallback, got %s", result.Fallbacks[0].Name)
	}
}

func TestPolicy_Evaluate_TaskPlan(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	policy := DefaultPolicy()
	result, err := policy.Evaluate("plan", 500, reg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Primary.Name != "qwen2.5-14b-instruct" {
		t.Fatalf("expected 14b for plan, got %s", result.Primary.Name)
	}
}

func TestPolicy_Evaluate_CtxTokensGT(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	policy := DefaultPolicy()
	result, err := policy.Evaluate("unknown", 7000, reg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Primary.Name != "qwen2.5-7b-instruct" {
		t.Fatalf("expected 7b for ctx>6000, got %s", result.Primary.Name)
	}
}

func TestPolicy_Evaluate_Default(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	policy := DefaultPolicy()
	result, err := policy.Evaluate("unknown", 100, reg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Primary.Name != "qwen2.5-7b-instruct" {
		t.Fatalf("expected 7b default, got %s", result.Primary.Name)
	}
	if result.RuleIdx != -1 {
		t.Fatalf("expected ruleIdx=-1 for default, got %d", result.RuleIdx)
	}
}

func TestRouter_RouteNoCall(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	policy := DefaultPolicy()
	router := NewRouter(reg, policy, nil)

	result, err := router.Route("title", 200)
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if result.Primary.Name != "qwen2.5-3b-instruct" {
		t.Fatalf("expected 3b, got %s", result.Primary.Name)
	}
}

func TestRouter_RecentRoutes(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	policy := DefaultPolicy()
	router := NewRouter(reg, policy, nil)

	// 手动注入几条记录.
	router.record(RouteRecord{Task: "title", Chosen: "3b", Success: true})
	router.record(RouteRecord{Task: "plan", Chosen: "14b", Success: true})

	recent := router.RecentRoutes(10)
	if len(recent) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recent))
	}
	// 最新的在前.
	if recent[0].Task != "plan" {
		t.Fatalf("expected plan first, got %s", recent[0].Task)
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// failClient 总是返回错误.
type failClient struct{}

func (c *failClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("model unavailable")
}

func (c *failClient) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	return nil, errors.New("not implemented")
}

// successClient 总是返回成功.
type successClient struct {
	model string
}

func (c *successClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	return ChatResponse{
		Message:      Message{Role: RoleAssistant, Content: "ok"},
		FinishReason: "stop",
		Usage:        Usage{TotalTokens: 10},
		Model:        c.model,
	}, nil
}

func (c *successClient) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func TestRouter_FallbackSuccess(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	policy := DefaultPolicy()
	// primary (3b) 失败, fallback (7b) 成功.
	router := NewRouter(reg, policy, &successClient{})

	// 用 failClient 覆盖 3b, 用 successClient 覆盖 7b.
	router.clients["qwen2.5-3b-instruct"] = &failClient{}
	router.clients["qwen2.5-7b-instruct"] = &successClient{}

	resp, rec, err := router.ChatForTask(context.Background(), "title", ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}
	if rec.Chosen != "qwen2.5-7b-instruct" {
		t.Fatalf("expected chosen=7b, got %s", rec.Chosen)
	}
	if len(rec.Fallbacks) != 1 {
		t.Fatalf("expected 1 fallback tried, got %d", len(rec.Fallbacks))
	}
	if !rec.Success {
		t.Fatal("should be success")
	}
	_ = resp
}

func TestRouter_AllFail(t *testing.T) {
	reg := DefaultRegistry("http://localhost:8080/v1", "L")
	policy := DefaultPolicy()
	router := NewRouter(reg, policy, &failClient{})

	_, rec, err := router.ChatForTask(context.Background(), "title", ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error when all models fail")
	}
	if rec.Success {
		t.Fatal("should be failure")
	}
	if len(rec.Fallbacks) != 2 {
		t.Fatalf("expected 2 models tried (primary + 1 fallback), got %d", len(rec.Fallbacks))
	}
}
