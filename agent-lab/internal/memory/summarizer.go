package memory

import (
	"context"
	"fmt"
	"strings"

	"ai-learn-playground/agent-lab/internal/llm"
)

// Summarizer 把一段历史消息压成一条摘要, 供 ShortTerm 在越界时替换最旧若干轮.
//
// 独立成类型 (M4) 是为了:
//   - 单测可以直接构造 Summarizer + 假 client 验证 "摘要被调用";
//   - 后续里程碑 (M6 planner) 复用同一份压缩逻辑.
type Summarizer struct {
	client    llm.Client
	model     string
	maxTokens int
}

// NewSummarizer 构造一个摘要器. maxTokens 限制摘要输出长度.
func NewSummarizer(client llm.Client, model string, maxTokens int) *Summarizer {
	if maxTokens <= 0 {
		maxTokens = 256
	}
	return &Summarizer{client: client, model: model, maxTokens: maxTokens}
}

// Summarize 把 msgs 压成一段不超过 maxTokens 的中文摘要.
// 调用方负责挑选要压缩的子集 (通常是最旧的一半).
func (s *Summarizer) Summarize(ctx context.Context, msgs []llm.Message) (string, error) {
	if s == nil || s.client == nil {
		// 没有 client: 退化成把消息拼成一句占位, 不丢商品关键字.
		return fallbackSummary(msgs), nil
	}
	var b strings.Builder
	b.WriteString("请把下面这些旧对话压缩成一段简短摘要. 保留关键商品信息和风格关键字. 不要超过 300 字.\n\n")
	for _, m := range msgs {
		fmt.Fprintf(&b, "%s: %s\n", m.Role, m.Content)
	}
	req := llm.ChatRequest{
		Model: s.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "你是一个简洁的摘要器."},
			{Role: llm.RoleUser, Content: b.String()},
		},
		Temperature: float32Ptr(0.3),
		MaxTokens:   intPtr(s.maxTokens),
		Stream:      false,
	}
	resp, err := s.client.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("summarize: %w", err)
	}
	summary := strings.TrimSpace(resp.Message.Content)
	if summary == "" {
		summary = fallbackSummary(msgs)
	}
	return summary, nil
}

// fallbackSummary 在没有 LLM 或 LLM 失败时, 把消息首句拼起来, 至少不丢上下文.
func fallbackSummary(msgs []llm.Message) string {
	if len(msgs) == 0 {
		return "[旧对话已压缩]"
	}
	var b strings.Builder
	b.WriteString("[历史摘要] ")
	for i, m := range msgs {
		if i >= 4 {
			b.WriteString("…")
			break
		}
		first := firstLineOf(m.Content)
		if first == "" {
			continue
		}
		if b.Len() > len("[历史摘要] ") {
			b.WriteString(" / ")
		}
		b.WriteString(first)
	}
	out := b.String()
	if out == "[历史摘要] " {
		return "[旧对话已压缩]"
	}
	return out
}

func firstLineOf(s string) string {
	if idx := strings.IndexAny(s, "\n\r"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}
