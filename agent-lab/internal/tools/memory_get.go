package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/memory"
)

// MemoryGet 让 agent 读取长期记忆里的一个键.
//
// 典型用法: 在为某卖家写文案前, 先 memory_get(namespace="seller:A001", key="tone")
// 拿到该卖家偏好的口吻, 命中跨会话的个性化.
type MemoryGet struct {
	kv *memory.KV
}

// NewMemoryGet 构造一个绑定到长期记忆 KV 的读取工具.
func NewMemoryGet(kv *memory.KV) *MemoryGet {
	return &MemoryGet{kv: kv}
}

// Schema 实现 Tool.
func (m *MemoryGet) Schema() llm.ToolSchema {
	return Schema(
		"memory_get",
		"从长期记忆读取一个键. namespace 通常是 'seller:{卖家ID}', key 通常是 'tone' (口吻偏好) 或 'keywords' (常用关键词). 返回 JSON: {found, value}. 写文案前先读 seller 的 tone 以保持口吻一致.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "记忆分区, 例如 seller:A001"},
				"key":       map[string]any{"type": "string", "description": "键名, 例如 tone / keywords"},
			},
			"required": []string{"namespace", "key"},
		},
	)
}

type memoryGetArgs struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

// Invoke 实现 Tool.
func (m *MemoryGet) Invoke(ctx context.Context, raw json.RawMessage) (string, error) {
	var args memoryGetArgs
	if err := ParseArgs(raw, &args); err != nil {
		return "", err
	}
	if args.Namespace == "" || args.Key == "" {
		return "", fmt.Errorf("namespace and key are required")
	}
	value, found, err := m.kv.Get(ctx, args.Namespace, args.Key)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(map[string]any{
		"found": found,
		"value": value,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}
