package trace

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ai-learn-playground/agent-lab/internal/store"
)

func newTestRecorder(t *testing.T) *Recorder {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewRecorder(st)
}

func TestRecorder_TraceAndSpans(t *testing.T) {
	rec := newTestRecorder(t)
	ctx := context.Background()

	tr, ctx := rec.NewTrace(ctx, "conv_1", "为毛巾写文案")
	if tr.TraceID == "" {
		t.Fatal("trace ID should not be empty")
	}
	if tr.Status != "running" {
		t.Fatalf("status=%s, want running", tr.Status)
	}

	// LLM span.
	s1 := rec.StartSpan(ctx, SpanLLM, "planner.Chat")
	s1.TokensIn = 100
	s1.TokensOut = 50
	s1.Output = `{"goal":"test"}`
	rec.EndSpan(ctx, s1)

	// Tool span.
	s2 := rec.StartSpan(ctx, SpanTool, "product_lookup")
	s2.Output = `{"sku":"001"}`
	rec.EndSpan(ctx, s2)

	rec.FinishTrace(ctx, tr.TraceID, "ok")

	// 查询.
	got, err := rec.GetTrace(ctx, tr.TraceID)
	if err != nil {
		t.Fatalf("get trace: %v", err)
	}
	if got.Status != "ok" {
		t.Fatalf("status=%s, want ok", got.Status)
	}
	if len(got.Spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(got.Spans))
	}
	if got.Spans[0].Kind != SpanLLM {
		t.Fatalf("span 0 kind=%s, want llm", got.Spans[0].Kind)
	}
	if got.Spans[0].TokensIn != 100 {
		t.Fatalf("tokens_in=%d", got.Spans[0].TokensIn)
	}
	if got.Spans[1].Kind != SpanTool {
		t.Fatalf("span 1 kind=%s, want tool", got.Spans[1].Kind)
	}
}

func TestRecorder_ListTraces(t *testing.T) {
	rec := newTestRecorder(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		tr, _ := rec.NewTrace(ctx, "conv", "goal "+string(rune('A'+i)))
		rec.FinishTrace(ctx, tr.TraceID, "ok")
	}

	traces, err := rec.ListTraces(ctx, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(traces) != 3 {
		t.Fatalf("expected 3 traces, got %d", len(traces))
	}
}

func TestRecorder_PersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	rec := NewRecorder(st)
	tr, ctx := rec.NewTrace(ctx, "c1", "test goal")
	s := rec.StartSpan(ctx, SpanStep, "step1")
	rec.EndSpan(ctx, s)
	rec.FinishTrace(ctx, tr.TraceID, "ok")
	st.Close()

	st2, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	rec2 := NewRecorder(st2)
	got, err := rec2.GetTrace(ctx, tr.TraceID)
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if got.Goal != "test goal" {
		t.Fatalf("goal=%s", got.Goal)
	}
	if len(got.Spans) != 1 {
		t.Fatalf("spans=%d after reopen", len(got.Spans))
	}
}

func TestContextPropagation(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "tr_test")
	got, ok := TraceIDFromContext(ctx)
	if !ok || got != "tr_test" {
		t.Fatalf("trace ID not propagated: %s", got)
	}

	ctx = WithSpanID(ctx, "sp_parent")
	parent, ok := SpanIDFromContext(ctx)
	if !ok || parent != "sp_parent" {
		t.Fatalf("span ID not propagated: %s", parent)
	}
}

func TestSpanDuration(t *testing.T) {
	s := &Span{
		StartedAt: time.Now(),
	}
	time.Sleep(10 * time.Millisecond)
	s.EndedAt = time.Now()
	d := s.Duration()
	if d < 8*time.Millisecond {
		t.Fatalf("duration too short: %v", d)
	}
}
