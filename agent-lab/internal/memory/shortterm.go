// Package memory 管理短期会话记忆: 历史消息+token 预算+超出时摘要压缩.
//
// 目标: 在没有向量索引时靠纯文本滑窗 + LLM 摘要, 把上下文压到预算内.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
)

// ShortTerm 是一次会话的短期记忆.
//
// 不变量:
//  budget 是 totalTokens <= budget; reserve 留给输出 tokens 预留.
// system prompt 单独存储, 不计入 msgs 的预算(会单独估算字符估算  估算 tokens.
type ShortTerm struct {
	system  string
	msgs    []llm.Message
	budget  int
	reserve int
}

// NewShortTerm 构造一个空的短期记忆, budget=总 token 预算, reserve=输出预留.
func NewShortTerm(system string, budget, reserve int) *ShortTerm {
	if budget <= 0 {
		budget = 2048
	}
	if reserve <= 0 {
		reserve = 512
	}
	return &ShortTerm{system: system, budget: budget, reserve: reserve}
}

// SetSystem 覆盖 system prompt, 不会影响历史消息.
func (m *ShortTerm) SetSystem(s string) { m.system = s }

// System 返回当前 system prompt.
func (m *ShortTerm) System() string { return m.system }

// Append 追加一条消息 (user 或 assistant).
func (m *ShortTerm) Append(role llm.Role, content string) {
	if content == "" {
		return
	}
	m.msgs = append(m.msgs, llm.Message{Role: role, Content: content})
}

// Len 返回历史消息条数 (不含 system).
func (m *ShortTerm) Len() int { return len(m.msgs) }

// EstimateTokens 粗略估算一段中文文本的 token 数: 中文 1.5 字符 ≈ 1 token, 英文 4 字符 ≈ 1 token.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	var cjk, other int
	for _, r := range s {
		if isCJK(r) {
			cjk++
		} else {
			other++
		}
	}
	tokens := cjk*2/3 + other/4
	if tokens <= 0 {
		tokens = 1
	}
	return tokens
}

func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF: // CJK 统一表意
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK 扩展 A
		return true
	case r >= 0x3040 && r <= 0x30FF: // 平假/片假
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // 韩文
		return true
	}
	return false
}

// Snapshot 返回可直接喂给 LLM 的 messages slice: [system, msgs...].
// 调用方不应修改返回值.
func (m *ShortTerm) Snapshot() []llm.Message {
	out := make([]llm.Message, 0, 1+len(m.msgs))
	out = append(out, llm.Message{Role: llm.RoleSystem, Content: m.system})
	out = append(out, m.msgs...)
	return out
}

// CompressInfo 记录一次压缩的结果信息, 用于 UI 展示.
type CompressInfo struct {
	DidCompress bool   // 是否发生了压缩
	BeforeTurns int    // 压缩前轮数
	AfterTurns  int    // 压缩后轮数
	BeforeChars int    // 压缩前字符数
	Summary     string // 摘要内容 (若未压缩为空)
}

// EnsureBudget 检查当前历史 (不含最新一条) 的 token 估算是否超预算,
// 如果超出则用 LLM 把最旧若干轮摘要成一条 assistant message.
//
// 策略:
//  1. 先尝试简单滑窗: 如果最旧的一条丢掉后 <= budget-reserve, 直接丢.
//  2. 如果还是超, 调 LLM 摘要最旧一半轮为一条 summary 并替换.
//
// client 可能为 nil, 此时降级为简单滑窗.
func (m *ShortTerm) EnsureBudget(ctx context.Context, client llm.Client, model string, maxTokens int) (CompressInfo, error) {
	available := m.budget - m.reserve
	current := m.estimateAllTokens()
	info := CompressInfo{BeforeTurns: len(m.msgs), BeforeChars: m.countChars()}
	if current <= available {
		return info, nil
	}
	// 滑窗: 持续丢掉最旧一轮直到 <= budget, 或者轮数 <= 6 (保留最近 6 轮).
	for len(m.msgs) > 6 && m.estimateAllTokens() > available {
		m.msgs = m.msgs[1:]
	}
	if m.estimateAllTokens() <= available {
		info.DidCompress = true
		info.AfterTurns = len(m.msgs)
		return info, nil
	}
	// LLM 摘要: 把最旧的一半合并为一条 summary.
	if client == nil {
		// 没有 client, 继续滑窗降级.
		for len(m.msgs) > 2 && m.estimateAllTokens() > available {
			m.msgs = m.msgs[2:]
		}
		info.DidCompress = true
		info.AfterTurns = len(m.msgs)
		return info, nil
	}
	half := len(m.msgs) / 2
	if half < 2 {
		// 只剩两条没法摘要了.
		return info, nil
	}
	oldHalf := make([]llm.Message, 0, half)
	copy(oldHalf, m.msgs[:half])
	// 构造摘要请求.
	summarize := "请把下面这些旧对话压缩成一段简短摘要. 保留关键商品信息和风格关键字. 不要超过 300 字.\n\n"
	for _, msg := range oldHalf {
		summarize += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}
	req := llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "你是一个简洁的摘要器."},
			{Role: llm.RoleUser, Content: summarize},
		},
		Temperature: float32Ptr(0.3),
		MaxTokens:   intPtr(maxTokens),
		Stream:      false,
	}
	resp, err := client.Chat(ctx, req)
	if err != nil {
		// 摘要失败, 退化为滑窗.
		for len(m.msgs) > 2 {
			m.msgs = m.msgs[2:]
		}
		info.DidCompress = true
		info.AfterTurns = len(m.msgs)
		return info, fmt.Errorf("summary fallback to sliding window: %w", err)
	}
	summary := strings.TrimSpace(resp.Message.Content)
	if summary == "" {
		summary = "[旧对话已压缩]"
	}
	// 用一条 assistant message 替换最旧 half 条.
	newMsgs := make([]llm.Message, 0, len(m.msgs)-half+1)
	newMsgs = append(newMsgs, llm.Message{Role: llm.RoleAssistant, Content: "[历史摘要] " + summary})
	newMsgs = append(newMsgs, m.msgs[half:]...)
	m.msgs = newMsgs
	info.DidCompress = true
	info.AfterTurns = len(m.msgs)
	info.Summary = summary
	return info, nil
}

// estimateAllTokens 估算 system + 所有历史消息的 token 数.
func (m *ShortTerm) estimateAllTokens() int {
	total := EstimateTokens(m.system)
	for _, msg := range m.msgs {
		total += EstimateTokens(msg.Content)
	}
	return total
}

func (m *ShortTerm) countChars() int {
	total := len(m.system)
	for _, msg := range m.msgs {
		total += len(msg.Content)
	}
	return total
}

// Reset 清空历史消息, 保留 system.
func (m *ShortTerm) Reset() { m.msgs = nil }

// ResetWith 用给定的 messages 重建历史 (不含 system).
func (m *ShortTerm) ResetWith(msgs []llm.Message) {
	m.msgs = make([]llm.Message, len(msgs))
	copy(m.msgs, msgs)
}

// Messages 返回历史消息 (不含 system), 供 save/load 使用.
func (m *ShortTerm) Messages() []llm.Message {
	out := make([]llm.Message, len(m.msgs))
	copy(out, m.msgs)
	return out
}

// SaveToFile 把 system + 历史存成 JSON, 方便持久化.
func (m *ShortTerm) SaveToFile(path string) error {
	data := struct {
		System  string        `json:"system"`
		Messages []llm.Message `json:"messages"`
		CreatedAt time.Time `json:"created_at"`
	}{
		System:     m.system,
		Messages:   m.msgs,
		CreatedAt:  time.Now(),
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// LoadFromFile 从 JSON 恢复 system + 历史.
func (m *ShortTerm) LoadFromFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var data struct {
		System  string        `json:"system"`
		Messages []llm.Message `json:"messages"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	m.system = data.System
	m.msgs = data.Messages
	return nil
}

func float32Ptr(v float32) *float32 { return &v }
func intPtr(v int) *int             { return &v }
