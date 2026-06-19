package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/rag"
	"ai-learn-playground/agent-lab/internal/store"
)

type testEmbedder struct{ dim int }

func (e *testEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, e.dim)
		for _, r := range t {
			v[int(r)%e.dim] += 1
		}
		out[i] = v
	}
	return out, nil
}

func (e *testEmbedder) Dim() int { return e.dim }

func newTestRetrieverWithDocs(t *testing.T, docs []string) *rag.Retriever {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	vs, err := memory.NewVectorStore(st)
	if err != nil {
		t.Fatalf("vs: %v", err)
	}
	emb := &testEmbedder{dim: 64}
	ctx := context.Background()
	for i, d := range docs {
		vec, _ := emb.Embed(ctx, []string{d})
		if err := vs.Add(ctx, "doc"+string(rune('0'+i)), "test", i, d, vec[0], "{}"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	return rag.NewRetriever(emb, vs)
}

func TestKBSearch_RequiresQuery(t *testing.T) {
	r := newTestRetrieverWithDocs(t, nil)
	tool := NewKBSearch(r)
	if _, err := tool.Invoke(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestKBSearch_ReturnsResults(t *testing.T) {
	docs := []string{"蝦皮標題120字", "momo標題60字", "小紅書標題20字"}
	r := newTestRetrieverWithDocs(t, docs)
	tool := NewKBSearch(r)
	out, err := tool.Invoke(context.Background(), json.RawMessage(`{"query":"蝦皮標題","k":3}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if res["count"].(float64) == 0 {
		t.Fatal("expected non-zero results")
	}
	results := res["results"].([]any)
	if len(results) == 0 {
		t.Fatal("expected results array")
	}
}
