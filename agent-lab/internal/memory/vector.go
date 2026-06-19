package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"ai-learn-playground/agent-lab/internal/store"
)

// VectorStore 是 RAG 的向量检索层 (M5).
//
// 策略 (ADR-0005 起步阶段):
//   - 持久化: 文档块 (text + embedding) 写入 SQLite documents 表.
//   - 检索: 启动时把全部向量加载进内存, 暴力遍历做 cosine top-k.
//   - 优点: 零额外依赖 (不用 sqlite-vec 扩展), 适合学习; 文档量 < 10k 时足够快.
//   - 后续 (ADR-0005 中段): 可换 sqlite-vec 做持久化 ANN, 接口不变.
type VectorStore struct {
	mu    sync.RWMutex
	docs  []store.DocWithVec
	store *store.Store
	dim   int
}

// NewVectorStore 构造一个绑定到 store 的向量库. Open 后自动从 SQLite hydrate.
func NewVectorStore(s *store.Store) (*VectorStore, error) {
	vs := &VectorStore{store: s}
	if s != nil {
		if err := vs.hydrate(); err != nil {
			return nil, fmt.Errorf("vector store hydrate: %w", err)
		}
	}
	return vs, nil
}

func (vs *VectorStore) hydrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	docs, err := vs.store.LoadDocs(ctx)
	if err != nil {
		return err
	}
	vs.mu.Lock()
	vs.docs = docs
	if len(docs) > 0 {
		vs.dim = len(docs[0].Embedding)
	}
	vs.mu.Unlock()
	if len(docs) > 0 {
		fmt.Printf("[vector] hydrated %d chunks (dim=%d)\n", len(docs), vs.dim)
	}
	return nil
}

// Add 写入一个文档块到 SQLite 并加入内存索引.
func (vs *VectorStore) Add(ctx context.Context, id, source string, chunkIndex int, text string, embedding []float32, metadata string) error {
	if vs.store != nil {
		if err := vs.store.PutDoc(ctx, id, source, chunkIndex, text, embedding, metadata); err != nil {
			return err
		}
	}
	vs.mu.Lock()
	// 去重: 如果 id 已存在则原地替换.
	found := false
	for i := range vs.docs {
		if vs.docs[i].ID == id {
			vs.docs[i] = store.DocWithVec{
				DocRow:    store.DocRow{ID: id, Source: source, ChunkIndex: chunkIndex, Text: text, Metadata: metadata, CreatedAt: time.Now().Unix()},
				Embedding: embedding,
			}
			found = true
			break
		}
	}
	if !found {
		vs.docs = append(vs.docs, store.DocWithVec{
			DocRow:    store.DocRow{ID: id, Source: source, ChunkIndex: chunkIndex, Text: text, Metadata: metadata, CreatedAt: time.Now().Unix()},
			Embedding: embedding,
		})
	}
	if vs.dim == 0 && len(embedding) > 0 {
		vs.dim = len(embedding)
	}
	vs.mu.Unlock()
	return nil
}

// DeleteBySource 删除某 source 的全部块 (内存 + SQLite).
func (vs *VectorStore) DeleteBySource(ctx context.Context, source string) error {
	if vs.store != nil {
		if err := vs.store.DeleteDocsBySource(ctx, source); err != nil {
			return err
		}
	}
	vs.mu.Lock()
	filtered := vs.docs[:0]
	for _, d := range vs.docs {
		if d.Source != source {
			filtered = append(filtered, d)
		}
	}
	vs.docs = filtered
	vs.mu.Unlock()
	return nil
}

// SearchResult 是一条检索结果.
type SearchResult struct {
	store.DocRow
	Score float32 `json:"score"`
}

// Search 对 query 向量做 brute-force cosine top-k.
func (vs *VectorStore) Search(query []float32, k int) []SearchResult {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	if k <= 0 {
		k = 5
	}
	type scored struct {
		idx   int
		score float32
	}
	results := make([]scored, 0, len(vs.docs))
	for i, d := range vs.docs {
		s := cosineSim(query, d.Embedding)
		results = append(results, scored{idx: i, score: s})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	if len(results) > k {
		results = results[:k]
	}
	out := make([]SearchResult, 0, len(results))
	for _, r := range results {
		out = append(out, SearchResult{
			DocRow: vs.docs[r.idx].DocRow,
			Score:  r.score,
		})
	}
	return out
}

// Count 返回当前内存中的文档块数.
func (vs *VectorStore) Count() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.docs)
}

// Dim 返回向量维度.
func (vs *VectorStore) Dim() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.dim
}

// Sources 返回所有 source 及块数.
func (vs *VectorStore) Sources(ctx context.Context) ([]store.DocSourceInfo, error) {
	if vs.store != nil {
		return vs.store.ListDocSources(ctx)
	}
	// 无 store (纯内存): 手动汇总.
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	m := make(map[string]int)
	for _, d := range vs.docs {
		m[d.Source]++
	}
	var out []store.DocSourceInfo
	for src, n := range m {
		out = append(out, store.DocSourceInfo{Source: src, Chunks: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out, nil
}

// cosineSim 计算两个向量的余弦相似度. 假设向量已归一化 (点积即余弦).
// 未归一化时自动除以模长, 保证结果在 [-1, 1].
func cosineSim(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}
