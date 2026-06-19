package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/rag"
)

// KBSearch 让 agent 用 RAG 检索知识库 (M5).
//
// 典型用法: agent 在写蝦皮标题前, 先 kb_search(query="蝦皮標題字數限制") 拿到平台规则,
// 再据此生成合规文案. 这比把所有规则塞进 system prompt 更省 token.
type KBSearch struct {
	retriever *rag.Retriever
}

// NewKBSearch 构造一个绑定到 RAG retriever 的知识库搜索工具.
func NewKBSearch(r *rag.Retriever) *KBSearch {
	return &KBSearch{retriever: r}
}

// Schema 实现 Tool.
func (k *KBSearch) Schema() llm.ToolSchema {
	return Schema(
		"kb_search",
		"检索知识库 (平台规则/商品手册). 在写文案前调用以获取平台规则 (如标题字数限制/违禁词/hashtag规范), 避免违规. 返回 top-k 相关文档块的 JSON.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "搜索关键词, 例如 '蝦皮標題字數' 或 'momo違禁詞'"},
				"k":     map[string]any{"type": "integer", "description": "返回结果数, 默认 5", "default": 5},
			},
			"required": []string{"query"},
		},
	)
}

type kbSearchArgs struct {
	Query string `json:"query"`
	K     int    `json:"k"`
}

// Invoke 实现 Tool.
func (k *KBSearch) Invoke(ctx context.Context, raw json.RawMessage) (string, error) {
	var args kbSearchArgs
	if err := ParseArgs(raw, &args); err != nil {
		return "", err
	}
	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	if args.K <= 0 {
		args.K = 5
	}
	results, err := k.retriever.Retrieve(ctx, args.Query, args.K)
	if err != nil {
		return "", err
	}
	return rag.RenderToolResponse(args.Query, results), nil
}
