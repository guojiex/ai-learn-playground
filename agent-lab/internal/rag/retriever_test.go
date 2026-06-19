package rag

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/store"
)

// fakeEmbedder 实现了 llm.Embedder, 把文本按 rune hash 成固定维度向量.
type fakeEmbedder struct{ dim int }

func (f *fakeEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		v := make([]float32, f.dim)
		for _, r := range text {
			v[int(r)%f.dim] += 1
		}
		out[i] = v
	}
	return out, nil
}

func (f *fakeEmbedder) Dim() int { return f.dim }

func newTestVectorStore(t *testing.T) (*memory.VectorStore, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	vs, err := memory.NewVectorStore(st)
	if err != nil {
		t.Fatalf("vector store: %v", err)
	}
	return vs, st
}

func TestRetriever_TopK(t *testing.T) {
	vs, _ := newTestVectorStore(t)
	emb := &fakeEmbedder{dim: 64}
	ctx := context.Background()

	docs := []string{
		"蝦皮標題字數上限120字元",
		"momo標題字數上限60字元",
		"小紅書標題建議20字以內",
		"PChome標題不可使用emoji",
	}
	for i, d := range docs {
		vec, _ := emb.Embed(ctx, []string{d})
		if err := vs.Add(ctx, "doc"+string(rune('0'+i)), "test", i, d, vec[0], "{}"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}

	r := NewRetriever(emb, vs)
	results, err := r.Retrieve(ctx, "蝦皮標題字數", 3)
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !strings.Contains(results[0].Text, "蝦皮") {
		t.Fatalf("top result should be about 蝦皮, got: %s", results[0].Text)
	}
}

func TestRetriever_Empty(t *testing.T) {
	vs, _ := newTestVectorStore(t)
	emb := &fakeEmbedder{dim: 64}
	r := NewRetriever(emb, vs)
	results, err := r.Retrieve(context.Background(), "anything", 5)
	if err != nil {
		t.Fatalf("retrieve on empty: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestRetrieveAndRender(t *testing.T) {
	vs, _ := newTestVectorStore(t)
	emb := &fakeEmbedder{dim: 64}
	ctx := context.Background()
	vec, _ := emb.Embed(ctx, []string{"測試文檔"})
	_ = vs.Add(ctx, "d1", "test", 0, "測試文檔內容", vec[0], "{}")

	r := NewRetriever(emb, vs)
	rendered, results, err := r.RetrieveAndRender(ctx, "測試", 5)
	if err != nil {
		t.Fatalf("retrieve and render: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if !strings.Contains(rendered, "检索结果") {
		t.Fatalf("rendered should contain header, got: %s", rendered)
	}
}

func TestRender_Empty(t *testing.T) {
	if Render(nil) != "" {
		t.Fatal("Render(nil) should be empty")
	}
}

func TestRenderToolResponse_ValidJSON(t *testing.T) {
	results := []memory.SearchResult{
		{DocRow: store.DocRow{Source: "s1", Text: "hello"}, Score: 0.9},
	}
	out := RenderToolResponse("test", results)
	if !strings.Contains(out, `"query"`) || !strings.Contains(out, `"hello"`) {
		t.Fatalf("invalid tool response: %s", out)
	}
}

var _ llm.Embedder = (*fakeEmbedder)(nil)
