// Package llm 定义与本地 OpenAI 兼容 server 通信的最小接口.
//
// 当前里程碑 (M0) 只提供 Chat / ChatStream. Embedding 留到 M5.
package llm

import (
	"context"
	"encoding/json"
)

// Role 是 OpenAI chat 消息中的 role 字段值.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall 与 OpenAI 协议保持一致, 用于后续 milestone (M2 起).
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"` // 通常为 "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall 描述 assistant 想调用的函数与序列化后的参数.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message 是一条 chat 消息.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolSchema 是注册给 LLM 的工具描述, 详细类型在 internal/tools 中给出 (M2).
type ToolSchema struct {
	Type     string         `json:"type"` // 固定为 "function"
	Function FunctionSchema `json:"function"`
}

// FunctionSchema 是 OpenAI 协议下的 function 描述.
type FunctionSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ChatRequest 描述一次 Chat 调用. 字段名贴近 OpenAI 协议.
type ChatRequest struct {
	Model       string       `json:"model"`
	Messages    []Message    `json:"messages"`
	Tools       []ToolSchema `json:"tools,omitempty"`
	Temperature *float32     `json:"temperature,omitempty"`
	TopP        *float32     `json:"top_p,omitempty"`
	MaxTokens   *int         `json:"max_tokens,omitempty"`
	Stop        []string     `json:"stop,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
}

// Usage 是模型返回的 token 计数, 字段为 omitempty 以兼容缺失.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// ChatResponse 是非流式调用的返回值.
type ChatResponse struct {
	Message      Message
	FinishReason string
	Usage        Usage
	Model        string
}

// StreamChunk 是流式调用每次推送的增量.
type StreamChunk struct {
	// DeltaContent 是本次增量的文本片段, 可能为空字符串.
	DeltaContent string
	// DeltaToolCalls 在流式 tool_calls 时使用 (M2 起再启用).
	DeltaToolCalls []ToolCall
	// FinishReason 在流末尾出现 (例如 "stop" / "tool_calls" / "length").
	FinishReason string
	// Usage 仅在 server 启用 stream usage 时出现 (llama.cpp 默认开启).
	Usage *Usage
	// Err 若非空表示流式过程中出错; 出错后 channel 会关闭.
	Err error
}

// Client 是 LLM 后端的最小接口. 后续里程碑会在另一个文件中扩展 Embedder 等.
type Client interface {
	// Chat 发起非流式 chat completion 请求.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	// ChatStream 发起流式 chat completion 请求.
	// 返回的 channel 在结束 / 出错 / ctx 取消时关闭.
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}
