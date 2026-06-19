// Package rag 提供 RAG (Retrieval-Augmented Generation) 的核心组件 (M5).
//
// 包含:
//   - chunker: 把长文本按字符数切成有重叠的块.
//   - retriever: embed query → 向量库 top-k → 返回结果.
//   - render: 把检索结果格式化成 system prompt 的知识上下文块.
package rag

import (
	"strings"
	"unicode/utf8"
)

// ChunkConfig 控制分块行为.
type ChunkConfig struct {
	ChunkSize int // 每块目标字符数 (按 rune 计), 默认 500.
	Overlap   int // 相邻块重叠字符数, 默认 50.
}

// DefaultChunkConfig 返回适合中文短文的默认配置.
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{ChunkSize: 500, Overlap: 50}
}

// Chunk 把 text 切成若干块. 按 rune 计数, 尽量在段落/句号边界切.
// 返回的每块都是 text 的子串, 不含前后空格.
func Chunk(text string, cfg ChunkConfig) []string {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 500
	}
	if cfg.Overlap < 0 {
		cfg.Overlap = 0
	}
	if cfg.Overlap >= cfg.ChunkSize {
		cfg.Overlap = cfg.ChunkSize / 4
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= cfg.ChunkSize {
		return []string{strings.TrimSpace(text)}
	}
	var chunks []string
	step := cfg.ChunkSize - cfg.Overlap
	for start := 0; start < len(runes); start += step {
		end := start + cfg.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[start:end])
		// 尝试在句号/换行处微调切点, 避免把句子从中间截断.
		if end < len(runes) {
			if boundary := findBoundary(chunk); boundary > cfg.ChunkSize/2 {
				chunk = chunk[:boundary]
				// 调整 start 使下一块从此处开始 (减 overlap).
				adjustedStart := start + boundary - cfg.Overlap
				if adjustedStart > start {
					start = adjustedStart - step
				}
			}
		}
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end >= len(runes) {
			break
		}
	}
	return chunks
}

// findBoundary 在 chunk 末尾附近找最后一个句子边界 (换行/句号/问号/感叹号).
func findBoundary(chunk string) int {
	runes := []rune(chunk)
	for i := len(runes) - 1; i >= len(runes)/2; i-- {
		switch runes[i] {
		case '\n', '。', '！', '？', '.', '!', '?':
			return i + 1
		}
	}
	return -1
}

// ChunkCount 估算 text 会被切成多少块 (供 ingest 进度展示).
func ChunkCount(text string, cfg ChunkConfig) int {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 500
	}
	n := utf8.RuneCountInString(strings.TrimSpace(text))
	if n == 0 {
		return 0
	}
	if n <= cfg.ChunkSize {
		return 1
	}
	step := cfg.ChunkSize - cfg.Overlap
	if step <= 0 {
		step = cfg.ChunkSize
	}
	return (n-1)/step + 1
}
