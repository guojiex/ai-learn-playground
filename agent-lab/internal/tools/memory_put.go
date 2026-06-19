package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/memory"
)

// MemoryPut 让 agent 把一条长期记忆写入 KV.
//
// 典型用法: 卖家说 "我喜欢闺蜜风, 多用 Emoji, 价格放最后" 时, agent 调用
// memory_put(namespace="seller:A001", key="tone", value={"style":"girlfriend",...}),
// 下次同卖家再开会话仍能命中.
type MemoryPut struct {
	kv *memory.KV
}

// NewMemoryPut 构造一个绑定到长期记忆 KV 的写入工具.
func NewMemoryPut(kv *memory.KV) *MemoryPut {
	return &MemoryPut{kv: kv}
}

// Schema 实现 Tool.
func (m *MemoryPut) Schema() llm.ToolSchema {
	return Schema(
		"memory_put",
		"向长期记忆写入 (或覆盖) 一个键. 当用户明确表达偏好 (口吻/关键词/禁忌) 时调用, 让后续会话能记住. value 必须是合法 JSON 字符串.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "记忆分区, 例如 seller:A001"},
				"key":       map[string]any{"type": "string", "description": "键名, 例如 tone / keywords"},
				"value":     map[string]any{"type": "string", "description": "要存的 JSON 值, 例如 {\"style\":\"girlfriend\",\"emoji\":\"high\"}"},
			},
			"required": []string{"namespace", "key", "value"},
		},
	)
}

type memoryPutArgs struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

// Invoke 实现 Tool.
func (m *MemoryPut) Invoke(ctx context.Context, raw json.RawMessage) (string, error) {
	var args memoryPutArgs
	if err := ParseArgs(raw, &args); err != nil {
		return "", err
	}
	if args.Namespace == "" || args.Key == "" {
		return "", fmt.Errorf("namespace and key are required")
	}
	if args.Value == "" {
		return "", fmt.Errorf("value is required")
	}
	// 校验 value 是合法 JSON, 避免把乱码塞进 KV.
	if !json.Valid([]byte(args.Value)) {
		return "", fmt.Errorf("value must be valid JSON")
	}
	if err := m.kv.Put(ctx, args.Namespace, args.Key, args.Value); err != nil {
		return "", err
	}
	out, err := json.Marshal(map[string]any{
		"ok":        true,
		"namespace": args.Namespace,
		"key":       args.Key,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}
