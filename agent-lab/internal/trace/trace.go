// Package trace 提供统一的 Trace/Span 可观测性 (M9).
//
// 一次 Run 是一个 trace, 每个 LLM/工具/agent step 调用是一个 span.
// 通过 context.Context 透传 trace_id, 任意 milestone 的 agent 都能接入.
package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"ai-learn-playground/agent-lab/internal/store"
)

// SpanKind 是 span 的类型.
type SpanKind string

const (
	SpanLLM   SpanKind = "llm"
	SpanTool  SpanKind = "tool"
	SpanStep  SpanKind = "step"
	SpanAgent SpanKind = "agent"
)

// Span 是一次 LLM/工具/agent 调用的记录.
type Span struct {
	SpanID    string    `json:"span_id"`
	TraceID   string    `json:"trace_id"`
	ParentID  string    `json:"parent_id,omitempty"`
	Kind      SpanKind  `json:"kind"`
	Name      string    `json:"name"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Attrs     string    `json:"attrs,omitempty"`  // JSON
	Input     string    `json:"input,omitempty"`  // JSON
	Output    string    `json:"output,omitempty"` // JSON
	TokensIn  int       `json:"tokens_in"`
	TokensOut int       `json:"tokens_out"`
	Error     string    `json:"error,omitempty"`
}

// Duration 返回 span 的耗时.
func (s *Span) Duration() time.Duration {
	if s.EndedAt.IsZero() {
		return time.Since(s.StartedAt)
	}
	return s.EndedAt.Sub(s.StartedAt)
}

// Trace 是一次完整 Run 的记录.
type Trace struct {
	TraceID   string    `json:"trace_id"`
	ConvID    string    `json:"conv_id"`
	Goal      string    `json:"goal"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Status    string    `json:"status"` // "running" | "ok" | "fail"
	Spans     []Span    `json:"spans,omitempty"`
}

// Recorder 负责创建和持久化 trace/span.
type Recorder struct {
	store *store.Store
	mu    sync.Mutex
}

// NewRecorder 构造一个绑定到 store 的 recorder.
// store 为 nil 时仅内存 (不持久化).
func NewRecorder(st *store.Store) *Recorder {
	return &Recorder{store: st}
}

// NewTrace 创建并持久化一个新 trace.
func (r *Recorder) NewTrace(ctx context.Context, convID, goal string) (*Trace, context.Context) {
	traceID := newID("tr")
	t := &Trace{
		TraceID:   traceID,
		ConvID:    convID,
		Goal:      goal,
		StartedAt: time.Now(),
		Status:    "running",
	}
	if r.store != nil {
		r.store.DB().ExecContext(ctx,
			`INSERT INTO traces(trace_id, conv_id, goal, started_at, status) VALUES(?,?,?,?,?)`,
			t.TraceID, t.ConvID, t.Goal, t.StartedAt.Unix(), t.Status)
	}
	return t, WithTraceID(ctx, traceID)
}

// FinishTrace 标记 trace 完成.
func (r *Recorder) FinishTrace(ctx context.Context, traceID, status string) {
	end := time.Now().Unix()
	if r.store != nil {
		r.store.DB().ExecContext(ctx,
			`UPDATE traces SET ended_at=?, status=? WHERE trace_id=?`,
			end, status, traceID)
	}
}

// StartSpan 创建一个新 span 并返回它 (需调 EndSpan 收尾).
func (r *Recorder) StartSpan(ctx context.Context, kind SpanKind, name string) *Span {
	traceID, _ := TraceIDFromContext(ctx)
	parentID, _ := SpanIDFromContext(ctx)
	s := &Span{
		SpanID:    newID("sp"),
		TraceID:   traceID,
		ParentID:  parentID,
		Kind:      kind,
		Name:      name,
		StartedAt: time.Now(),
	}
	return s
}

// EndSpan 持久化一个完成的 span.
func (r *Recorder) EndSpan(ctx context.Context, s *Span) {
	s.EndedAt = time.Now()
	if r.store != nil {
		r.store.DB().ExecContext(ctx,
			`INSERT INTO spans(span_id, trace_id, parent_id, kind, name, started_at, ended_at, attrs, input, output, tokens_in, tokens_out, error)
			 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			s.SpanID, s.TraceID, s.ParentID, string(s.Kind), s.Name,
			s.StartedAt.Unix(), s.EndedAt.Unix(),
			s.Attrs, s.Input, s.Output, s.TokensIn, s.TokensOut, s.Error)
	}
}

// --- context 透传 ---

type ctxKey int

const (
	keyTraceID ctxKey = iota
	keySpanID
)

// WithTraceID 把 trace_id 注入 context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, keyTraceID, traceID)
}

// TraceIDFromContext 从 context 取 trace_id.
func TraceIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyTraceID).(string)
	return v, ok
}

// WithSpanID 把 span_id 注入 context (作为子 span 的 parent).
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, keySpanID, spanID)
}

// SpanIDFromContext 从 context 取 parent span_id.
func SpanIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keySpanID).(string)
	return v, ok
}

// --- 查询 ---

// ListTraces 按时间倒序返回最近 N 条 trace.
func (r *Recorder) ListTraces(ctx context.Context, limit int) ([]Trace, error) {
	if r.store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.store.DB().QueryContext(ctx,
		`SELECT trace_id, conv_id, goal, started_at, ended_at, status
		 FROM traces ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list traces: %w", err)
	}
	defer rows.Close()
	var out []Trace
	for rows.Next() {
		var t Trace
		var started, ended int64
		if err := rows.Scan(&t.TraceID, &t.ConvID, &t.Goal, &started, &ended, &t.Status); err != nil {
			return nil, err
		}
		t.StartedAt = time.Unix(started, 0)
		if ended > 0 {
			t.EndedAt = time.Unix(ended, 0)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTrace 返回单个 trace + 其全部 spans.
func (r *Recorder) GetTrace(ctx context.Context, traceID string) (*Trace, error) {
	if r.store == nil {
		return nil, fmt.Errorf("store not configured")
	}
	var t Trace
	var started, ended int64
	err := r.store.DB().QueryRowContext(ctx,
		`SELECT trace_id, conv_id, goal, started_at, ended_at, status FROM traces WHERE trace_id=?`, traceID).
		Scan(&t.TraceID, &t.ConvID, &t.Goal, &started, &ended, &t.Status)
	if err != nil {
		return nil, fmt.Errorf("get trace: %w", err)
	}
	t.StartedAt = time.Unix(started, 0)
	if ended > 0 {
		t.EndedAt = time.Unix(ended, 0)
	}
	spans, err := r.ListSpans(ctx, traceID)
	if err != nil {
		return nil, err
	}
	t.Spans = spans
	return &t, nil
}

// ListSpans 按 started_at 排序返回某 trace 的全部 spans.
func (r *Recorder) ListSpans(ctx context.Context, traceID string) ([]Span, error) {
	if r.store == nil {
		return nil, nil
	}
	rows, err := r.store.DB().QueryContext(ctx,
		`SELECT span_id, trace_id, parent_id, kind, name, started_at, ended_at, attrs, input, output, tokens_in, tokens_out, error
		 FROM spans WHERE trace_id=? ORDER BY started_at`, traceID)
	if err != nil {
		return nil, fmt.Errorf("list spans: %w", err)
	}
	defer rows.Close()
	var out []Span
	for rows.Next() {
		var s Span
		var kind string
		var started, ended int64
		if err := rows.Scan(&s.SpanID, &s.TraceID, &s.ParentID, &kind, &s.Name,
			&started, &ended, &s.Attrs, &s.Input, &s.Output, &s.TokensIn, &s.TokensOut, &s.Error); err != nil {
			return nil, err
		}
		s.Kind = SpanKind(kind)
		s.StartedAt = time.Unix(started, 0)
		if ended > 0 {
			s.EndedAt = time.Unix(ended, 0)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// newID 生成带前缀的唯一 ID.
func newID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
