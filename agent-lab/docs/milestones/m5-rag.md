# M5 · RAG 检索增强

**前置**：M3 (推荐先 M4)  
**推荐档**：L  
**预计代码量**：~600 行

## 学习目标

- 起一个独立的 embedding server (bge-m3 / bge-small-zh)，跑通向量化全链路。
- 自己实现 chunking → embed → 检索 → rerank → citation 的最小 RAG。
- 学会在 prompt 里管理"检索结果"，避免被噪声片段带偏。

## 关键概念

- **Embedding**：把文本编码为定长向量。中文场景 bge-m3 或 bge-small-zh-v1.5 是稳定选择。
- **Chunking 策略**：
  - 固定窗口 + overlap (起手最简单)。
  - 按结构 (markdown 标题 / 段落) 切分，效果通常更好。
- **检索**：cosine 相似度。先在内存里做暴力 top-k，再切到 `sqlite-vec` 做持久化。
- **Rerank (轻量)**：用主 LLM 对 top-k 做"是否相关 (yes/no)" 二分类过滤，避免引入额外 reranker 模型。
- **Citation**：让模型在最终输出附带来源 id，工具可校验 id 是否真实存在。

## 要写的代码

```
agent-lab/
├── cmd/
│   ├── ingest/main.go           # 把 data/products + data/platform_rules 切片入库
│   └── agent/main.go            # 增加 --rag flag
├── internal/
│   ├── llm/
│   │   └── embed.go             # Embedder (调本地 embedding server)
│   ├── memory/
│   │   └── vector.go            # VectorStore 接口 + sqlite-vec 实现
│   ├── rag/
│   │   ├── chunker.go           # 段落 + overlap
│   │   ├── retriever.go         # top-k + 轻量 rerank
│   │   └── render.go            # 把检索结果格式化进 prompt
│   └── tools/
│       └── kb_search.go         # 暴露给 agent 的检索工具
└── data/
    └── platform_rules/
        ├── shopee_tw.md
        ├── momo.md
        └── xhs_tw.md
```

`VectorStore` 接口：

```go
type Doc struct {
    ID       string
    Source   string
    Chunk    string
    Vector   []float32
    Metadata map[string]string
}

type VectorStore interface {
    Upsert(ctx context.Context, docs []Doc) error
    Search(ctx context.Context, query []float32, topK int, filter map[string]string) ([]Doc, error)
}
```

## 业务表现

```text
$ go run ./agent-lab/cmd/ingest
ingested 6 products, 3 platform rules, total 138 chunks

$ go run ./agent-lab/cmd/agent --rag -m "蝦皮台湾对标题字数有什么要求? 顺手帮 sku_001 出一个标题"
[tool] kb_search({"q":"蝦皮 标题 字数","platform":"shopee_tw"}) -> 3 hits
[tool] product_lookup({"id":"sku_001"}) -> {...}
[tool] platform_lint({...}) -> {ok:true, len:54}
【日本製】今治本舗 純棉吸水浴巾 70x140 ... NT$690 (來源: shopee_tw.md#title-rules)
```

## UI 增量 (M5)

- **Knowledge 面板** (新)：搜索框 + top-k 命中片段卡片；点击片段查看完整原文 + metadata。
- Chat 面板里 assistant 输出中的 citation (`shopee_tw.md#title-rules` 这种) 渲染为可点击 chip，点击跳到 Knowledge 面板高亮对应片段。
- 入库进度显示：`cmd/ingest` 跑完后 UI 刷新可看到\"知识库: N chunks (来源: K 个文件)\"。

## 验收标准

- [ ] embedding server 与 chat server 各自一个端口，agent 都能命中。
- [ ] 入库支持增量：相同 (`source`, `chunk_id`) 再次入库会替换。
- [ ] top-k 检索 + 二阶段 rerank (LLM yes/no) 可分别 toggle，便于消融。
- [ ] 输出至少含 1 条 citation，且 citation 指向的 chunk 在库里存在。
- [ ] `kb_search` 支持 `filter` (按平台 / 按品类)。

## 进阶练习

1. 在 chunker 里实现"标题感知切分" (markdown headings → 块级切片)。
2. 把 cosine 换成"cosine + BM25 加权混合"。
3. 用主 LLM 对 retriever 命中做 LLM-as-Judge 评分，建立 RAG 质量回归集 (M9 复用)。

## 衔接

下一站 [M6](m6-planner.md)：让 Agent 先规划再执行，处理"上新一个 SKU"这种多步任务。
