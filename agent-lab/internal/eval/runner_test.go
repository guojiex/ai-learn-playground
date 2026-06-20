package eval

import (
	"context"
	"errors"
	"testing"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
)

// evalFakeClient 满足 llm.Client.
type evalFakeClient struct {
	responses []string
	calls     int
}

func (f *evalFakeClient) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	idx := f.calls
	f.calls++
	if idx < len(f.responses) {
		return llm.ChatResponse{
			Message:      llm.Message{Role: llm.RoleAssistant, Content: f.responses[idx]},
			FinishReason: "stop",
			Usage:        llm.Usage{PromptTokens: 50, CompletionTokens: 100, TotalTokens: 150},
		}, nil
	}
	return llm.ChatResponse{
		Message:      llm.Message{Role: llm.RoleAssistant, Content: "默认回复"},
		FinishReason: "stop",
		Usage:        llm.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}, nil
}

func (f *evalFakeClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func TestMetrics_SlangHit(t *testing.T) {
	m := NewMetrics()
	if hit := m.SlangHit("這款現貨免運 CP值超高 必買推薦"); hit < 0.6 {
		t.Fatalf("expected high slang hit, got %f", hit)
	}
	if hit := m.SlangHit("這是一個普通的文案沒有任何黑話"); hit > 0.1 {
		t.Fatalf("expected low slang hit, got %f", hit)
	}
	if hit := m.SlangHit(""); hit != 0 {
		t.Fatalf("empty text should have 0 hit, got %f", hit)
	}
}

func TestMetrics_ComplianceOK(t *testing.T) {
	m := NewMetrics()
	if !m.ComplianceOK("這款毛巾很好用", "shopee") {
		t.Fatal("clean text should be compliant")
	}
	if m.ComplianceOK("全網最便宜的毛巾", "shopee") {
		t.Fatal("banned term should not be compliant")
	}
	if m.ComplianceOK("保證治癒你的皮膚", "shopee") {
		t.Fatal("medical claim should not be compliant")
	}
}

func TestMetrics_SlangHits(t *testing.T) {
	m := NewMetrics()
	hits := m.SlangHits("現貨免運下殺")
	if len(hits) != 3 {
		t.Fatalf("expected 3 hits, got %d: %v", len(hits), hits)
	}
}

func TestMetrics_BannedHits(t *testing.T) {
	m := NewMetrics()
	hits := m.BannedHits("全網第一 最便宜")
	if len(hits) != 2 {
		t.Fatalf("expected 2 banned hits, got %d: %v", len(hits), hits)
	}
}

func TestJudge_ParseJSON(t *testing.T) {
	cases := []struct {
		input string
		score float64
		ok    bool
	}{
		{`{"score": 4, "reason": "good"}`, 4, true},
		{"```json\n{\"score\": 5}\n```", 5, true},
		{"评分如下:\n{\"score\": 3, \"reason\": \"一般\"}", 3, true},
		{"not json", 0, false},
	}
	for _, c := range cases {
		score, _, err := parseJudgeJSON(c.input)
		if (err == nil) != c.ok {
			t.Errorf("parseJudgeJSON(%q): ok=%v, want %v, err=%v", c.input, err == nil, c.ok, err)
		}
		if c.ok && score != c.score {
			t.Errorf("score=%f, want %f", score, c.score)
		}
	}
}

func TestRunner_Run(t *testing.T) {
	client := &evalFakeClient{
		responses: []string{
			// case 1 output
			"純棉浴巾 現貨免運 吸水快 蓬鬆柔軟 小資必買",
			// case 1 judge
			`{"score": 4, "reason": "卖点清晰"}`,
			// case 2 output
			"iPhone 15 手機壳 磁吸防摔 MagSafe",
			// case 2 judge
			`{"score": 3, "reason": "缺少黑话"}`,
		},
	}
	runner := NewRunner(client, "test-model", "test-model")
	cases := []Case{
		{ID: "e01", Prompt: "写浴巾标题", Platform: "shopee", Category: "居家"},
		{ID: "e02", Prompt: "写手机壳标题", Platform: "momo", Category: "3C"},
	}
	report, err := runner.Run(context.Background(), cases, "test", "baseline")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(report.Cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(report.Cases))
	}
	if report.OKCount != 2 {
		t.Fatalf("ok=%d, want 2", report.OKCount)
	}
	if report.MeanJudgeScore < 3 || report.MeanJudgeScore > 4 {
		t.Fatalf("mean judge score=%f, expected 3-4", report.MeanJudgeScore)
	}
}

func TestLoadCases(t *testing.T) {
	cases, err := LoadCases("testdata/cases.jsonl")
	if err != nil {
		// testdata 可能不存在, 跳过.
		t.Skip("testdata not available")
	}
	if len(cases) == 0 {
		t.Fatal("expected cases")
	}
}

func TestRenderMarkdown(t *testing.T) {
	report := &Report{
		Suite:     "ecom-v1",
		Tag:       "test",
		StartedAt: time.Now(),
		Cases: []CaseResult{
			{Case: Case{ID: "e01", Platform: "shopee"}, Output: "test", JudgeScore: 4, SlangHit: 0.8, ComplianceOK: true, TokensIn: 50, TokensOut: 100, LatencyMs: 200},
		},
		OKCount:        1,
		MeanJudgeScore: 4,
		MeanSlangHit:   0.8,
		ComplianceRate: 1.0,
		MeanTokens:     150,
		MeanLatencyMs:  200,
	}
	md := RenderMarkdown(report)
	if !contains(md, "Eval Report") || !contains(md, "e01") {
		t.Fatalf("markdown missing key content: %s", md[:100])
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
