package memory

import (
	"context"
	"path/filepath"
	"testing"

	"ai-learn-playground/agent-lab/internal/store"
)

func TestVectorStore_AddSearchDelete(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	vs, err := NewVectorStore(st)
	if err != nil {
		t.Fatalf("new vs: %v", err)
	}
	ctx := context.Background()

	// 添加 3 个文档块.
	docs := []struct {
		id   string
		text string
		vec  []float32
	}{
		{"d1#0", "蝦皮標題", []float32{1, 0, 0, 0}},
		{"d2#0", "momo標題", []float32{0, 1, 0, 0}},
		{"d3#0", "小紅書標題", []float32{0, 0, 1, 0}},
	}
	for _, d := range docs {
		src := string([]rune(d.text)[:2])
		if err := vs.Add(ctx, d.id, src, 0, d.text, d.vec, "{}"); err != nil {
			t.Fatalf("add %s: %v", d.id, err)
		}
	}
	if vs.Count() != 3 {
		t.Fatalf("count=%d, want 3", vs.Count())
	}

	// 搜索: query=[1,0,0,0] 应该最匹配 d1.
	results := vs.Search([]float32{1, 0, 0, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("results=%d, want 2", len(results))
	}
	if results[0].Text != "蝦皮標題" {
		t.Fatalf("top result=%q, want 蝦皮標題", results[0].Text)
	}

	// 删除 source "蝦皮".
	if err := vs.DeleteBySource(ctx, "蝦皮"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if vs.Count() != 2 {
		t.Fatalf("after delete count=%d, want 2", vs.Count())
	}
	results = vs.Search([]float32{1, 0, 0, 0}, 5)
	for _, r := range results {
		if r.Source == "蝦皮" {
			t.Fatal("蝦皮 should be deleted")
		}
	}
}

func TestVectorStore_PersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	vs, _ := NewVectorStore(st)
	_ = vs.Add(ctx, "x1", "test", 0, "hello", []float32{1, 0, 0}, "{}")
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// 重开: vector store 应从 SQLite hydrate.
	st2, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	vs2, err := NewVectorStore(st2)
	if err != nil {
		t.Fatalf("new vs2: %v", err)
	}
	if vs2.Count() != 1 {
		t.Fatalf("after reopen count=%d, want 1", vs2.Count())
	}
	results := vs2.Search([]float32{1, 0, 0}, 1)
	if len(results) != 1 || results[0].Text != "hello" {
		t.Fatalf("search after reopen: %+v", results)
	}
}

func TestCosineSim(t *testing.T) {
	// 相同向量 → 1.
	if s := cosineSim([]float32{1, 0}, []float32{1, 0}); s < 0.99 {
		t.Fatalf("identical vectors should have sim ~1, got %f", s)
	}
	// 正交向量 → 0.
	if s := cosineSim([]float32{1, 0}, []float32{0, 1}); s > 0.01 {
		t.Fatalf("orthogonal vectors should have sim ~0, got %f", s)
	}
	// 不同维度 → 0.
	if s := cosineSim([]float32{1, 0}, []float32{1}); s != 0 {
		t.Fatalf("mismatched dims should return 0, got %f", s)
	}
}
