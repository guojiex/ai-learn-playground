package rag

import (
	"context"
	"fmt"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/store"
)

// Retriever 把 "用户问题" 变成 "top-k 文档块" (M5).
//
// 流程: embed(query) → VectorStore.Search → []SearchResult.
type Retriever struct {
	embedder llm.Embedder
	vs       *memory.VectorStore
}

// NewRetriever 构造一个检索器.
func NewRetriever(embedder llm.Embedder, vs *memory.VectorStore) *Retriever {
	return &Retriever{embedder: embedder, vs: vs}
}

// Retrieve 对 query 做向量检索, 返回 top-k 结果.
// k <= 0 时默认 5.
func (r *Retriever) Retrieve(ctx context.Context, query string, k int) ([]memory.SearchResult, error) {
	if r == nil || r.vs == nil {
		return nil, fmt.Errorf("retriever not configured")
	}
	if r.embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}
	if k <= 0 {
		k = 5
	}
	vecs, err := r.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return r.vs.Search(vecs[0], k), nil
}

// RetrieveAndRender 是 Retrieve + Render 的便捷组合, 供 kb_search 工具直接用.
func (r *Retriever) RetrieveAndRender(ctx context.Context, query string, k int) (string, []memory.SearchResult, error) {
	results, err := r.Retrieve(ctx, query, k)
	if err != nil {
		return "", nil, err
	}
	return Render(results), results, nil
}

// Count 返回向量库文档块数 (委托给 VectorStore).
func (r *Retriever) Count() int {
	if r == nil || r.vs == nil {
		return 0
	}
	return r.vs.Count()
}

// Dim 返回向量维度.
func (r *Retriever) Dim() int {
	if r == nil || r.vs == nil {
		return 0
	}
	return r.vs.Dim()
}

// Sources 返回所有 source 及块数.
func (r *Retriever) Sources(ctx context.Context) ([]store.DocSourceInfo, error) {
	if r == nil || r.vs == nil {
		return nil, nil
	}
	return r.vs.Sources(ctx)
}
