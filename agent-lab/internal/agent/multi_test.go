package agent

import (
	"context"
	"errors"
	"testing"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// multiFakeClient 满足 llm.Client, 按调用次数返回不同回复 (模拟多轮对话).
type multiFakeClient struct {
	responses []string
	calls     int
}

func (f *multiFakeClient) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	idx := f.calls
	f.calls++
	if idx < len(f.responses) {
		return llm.ChatResponse{
			Message:      llm.Message{Role: llm.RoleAssistant, Content: f.responses[idx]},
			FinishReason: "stop",
			Usage:        llm.Usage{TotalTokens: 20},
		}, nil
	}
	return llm.ChatResponse{
		Message:      llm.Message{Role: llm.RoleAssistant, Content: `{"approve":true}`},
		FinishReason: "stop",
		Usage:        llm.Usage{TotalTokens: 10},
	}, nil
}

func (f *multiFakeClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func TestParseRoleJSON(t *testing.T) {
	cases := []struct {
		input string
		ok    bool
	}{
		{`{"approve": true, "issues": []}`, true},
		{"```json\n{\"approve\": false}\n```", true},
		{"好的, 以下是评审:\n{\"approve\": true}", true},
		{"not json at all", false},
	}
	for _, c := range cases {
		_, err := ParseRoleJSON(c.input)
		if (err == nil) != c.ok {
			t.Errorf("ParseRoleJSON(%q): ok=%v, want %v, err=%v", c.input, err == nil, c.ok, err)
		}
	}
}

func TestIsApproved(t *testing.T) {
	if !IsApproved(map[string]any{"approve": true}) {
		t.Fatal("approve=true should be approved")
	}
	if IsApproved(map[string]any{"approve": false}) {
		t.Fatal("approve=false should not be approved")
	}
	if IsApproved(map[string]any{}) {
		t.Fatal("missing approve should not be approved")
	}
}

func TestGetIssues(t *testing.T) {
	parsed := map[string]any{"issues": []any{"标题太短", "标签不够"}}
	issues := GetIssues(parsed, "issues")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0] != "标题太短" {
		t.Fatalf("issue 0=%s", issues[0])
	}
}

func TestMultiAgent_ApproveRound1(t *testing.T) {
	client := &multiFakeClient{
		responses: []string{
			`{"facts":["吸水","蓬鬆"],"summary":"今治毛巾"}`,
			`{"title":"今治毛巾","body":"蓬鬆吸水","tags":["#毛巾"]}`,
			`{"approve":true,"issues":[]}`,
			`{"approve":true,"violations":[]}`,
		},
	}
	reg := tools.NewRegistry()
	bus := NewMessageBus("test1", nil)
	multi := NewMultiAgent(client, reg, "test-model", bus)

	result, err := multi.Run(context.Background(), "为毛巾写文案", nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status=%s, want ok", result.Status)
	}
	if result.Rounds != 1 {
		t.Fatalf("rounds=%d, want 1 (approved in round 1)", result.Rounds)
	}
	if result.TotalTokens <= 0 {
		t.Fatal("expected positive token count")
	}
	if len(result.Results) != 4 {
		t.Fatalf("expected 4 step results, got %d", len(result.Results))
	}
}

func TestMultiAgent_RejectThenApprove(t *testing.T) {
	client := &multiFakeClient{
		responses: []string{
			// round 1
			`{"facts":["吸水"],"summary":"毛巾"}`,
			`{"title":"毛巾","body":"好","tags":["#毛巾"]}`,
			`{"approve":false,"issues":["标题太短"]}`,
			`{"approve":true,"violations":[]}`,
			// round 2
			`{"facts":["吸水","蓬鬆"],"summary":"今治毛巾"}`,
			`{"title":"日本今治毛巾 蓬鬆吸水","body":"蓬鬆吸水好","tags":["#毛巾","#日本"]}`,
			`{"approve":true,"issues":[]}`,
			`{"approve":true,"violations":[]}`,
		},
	}
	reg := tools.NewRegistry()
	bus := NewMessageBus("test2", nil)
	multi := NewMultiAgent(client, reg, "test-model", bus)

	result, err := multi.Run(context.Background(), "为毛巾写文案", nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status=%s, want ok", result.Status)
	}
	if result.Rounds != 2 {
		t.Fatalf("rounds=%d, want 2 (approved in round 2)", result.Rounds)
	}
}

func TestMultiAgent_MaxRounds(t *testing.T) {
	client := &multiFakeClient{
		responses: []string{},
	}
	// 所有轮次 critic 都不 approve.
	reg := tools.NewRegistry()
	bus := NewMessageBus("test3", nil)
	multi := NewMultiAgent(client, reg, "test-model", bus)
	multi.SetMaxRounds(2)

	// 覆盖 client 让 critic 总是 reject.
	client.responses = nil
	client.calls = 0
	rejectClient := &alwaysRejectClient{}
	multi2 := NewMultiAgent(rejectClient, reg, "test-model", bus)
	multi2.SetMaxRounds(2)

	result, err := multi2.Run(context.Background(), "为毛巾写文案", nil)
	_ = multi // avoid unused
	if err != nil {
		// max rounds returns nil error (not an error, just status)
	}
	if result.Status != "max_rounds" {
		t.Fatalf("status=%s, want max_rounds", result.Status)
	}
	if result.Rounds != 2 {
		t.Fatalf("rounds=%d, want 2", result.Rounds)
	}
}

// alwaysRejectClient 总是返回 reject.
type alwaysRejectClient struct{}

func (c *alwaysRejectClient) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{
		Message:      llm.Message{Role: llm.RoleAssistant, Content: `{"approve":false,"issues":["不够好"]}`},
		FinishReason: "stop",
		Usage:        llm.Usage{TotalTokens: 10},
	}, nil
}

func (c *alwaysRejectClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func TestTextSimilarity(t *testing.T) {
	if s := textSimilarity("hello world", "hello world"); s != 1.0 {
		t.Fatalf("identical text should have sim=1.0, got %f", s)
	}
	if s := textSimilarity("hello", "world"); s > 0.3 {
		t.Fatalf("different text should have low sim, got %f", s)
	}
	if s := textSimilarity("", "hello"); s != 0 {
		t.Fatalf("empty text should have sim=0, got %f", s)
	}
}
