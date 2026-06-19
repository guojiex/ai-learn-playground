package rag

import (
	"fmt"
	"strings"

	"ai-learn-playground/agent-lab/internal/memory"
)

// Render 把检索结果格式化成可注入 system prompt 的知识上下文块.
//
// 格式:
//
//	--- 知识库检索结果 (query 相关, 共 N 条) ---
//	[1] (score=0.82, source=shopee_rules.md)
//	<chunk text>
//
//	[2] (score=0.75, source=momo_rules.md)
//	<chunk text>
//	--- END ---
func Render(results []memory.SearchResult) string {
	if len(results) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "--- 知识库检索结果 (共 %d 条) ---\n", len(results))
	for i, r := range results {
		fmt.Fprintf(&b, "\n[%d] (score=%.2f, source=%s)\n", i+1, r.Score, r.Source)
		b.WriteString(r.Text)
		b.WriteString("\n")
	}
	b.WriteString("--- END ---")
	return b.String()
}

// RenderToolResponse 把检索结果格式化成 kb_search 工具的返回 JSON,
// 供 agent 在 tool message 里读到结构化结果.
func RenderToolResponse(query string, results []memory.SearchResult) string {
	var b strings.Builder
	b.WriteString("{\n  \"query\": ")
	appendJSONString(&b, query)
	b.WriteString(",\n  \"count\": ")
	fmt.Fprintf(&b, "%d", len(results))
	b.WriteString(",\n  \"results\": [")
	for i, r := range results {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\"score\": ")
		fmt.Fprintf(&b, "%.4f", r.Score)
		b.WriteString(", \"source\": ")
		appendJSONString(&b, r.Source)
		b.WriteString(", \"text\": ")
		appendJSONString(&b, r.Text)
		b.WriteString("}")
	}
	if len(results) > 0 {
		b.WriteString("\n  ]")
	} else {
		b.WriteString("]")
	}
	b.WriteString("\n}")
	return b.String()
}

func appendJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				fmt.Fprintf(b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
