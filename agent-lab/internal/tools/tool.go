// Package tools 定义工具接口与注册表, 给 agent 在 tool calling 阶段使用.
//
// 设计目标 (M2):
//   - 一个 Tool 既能给出自描述 schema (喂给 LLM), 又能在拿到 JSON args 时执行.
//   - Registry 是进程内只读注册表, 在启动时 Register 完成后即不再变更.
//   - 所有工具的输入与输出都用字符串 / JSON, 不引入跨进程依赖.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"ai-learn-playground/agent-lab/internal/llm"
)

// Tool 是 agent 可调用的一个工具.
//
// 实现者 SHOULD:
//   - 在 Schema().Function.Description 中用面向模型的视角写: 何时该用、典型参数.
//   - 在 Invoke 中对 args 做严格 JSON 反序列化, 失败时返回 error.
//   - 返回值使用结构化 JSON 字符串, 方便模型解析与回填.
type Tool interface {
	Schema() llm.ToolSchema
	Invoke(ctx context.Context, args json.RawMessage) (string, error)
}

// RiskLevel 表示工具的风险等级 (M8 HITL).
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"    // 可逆: 查询/生成文案, 无需审批
	RiskMedium RiskLevel = "medium" // 半可逆: 修改配置/价格, 可选审批
	RiskHigh   RiskLevel = "high"   // 不可逆: 发布/改库存, 强制审批
)

// RiskLeveler 是 Tool 可选实现的接口, 返回工具的风险等级.
// 未实现时默认 RiskLow.
type RiskLeveler interface {
	RiskLevel() RiskLevel
}

// GetRiskLevel 安全地获取工具的风险等级, 未实现 RiskLeveler 时返回 RiskLow.
func GetRiskLevel(t Tool) RiskLevel {
	if r, ok := t.(RiskLeveler); ok {
		return r.RiskLevel()
	}
	return RiskLow
}

// Registry 是一个并发安全的 Tool 注册表.
type Registry struct {
	mu    sync.RWMutex
	items map[string]Tool
}

// NewRegistry 构造一个空注册表.
func NewRegistry() *Registry {
	return &Registry{items: make(map[string]Tool)}
}

// Register 注册一个工具. 重名直接覆盖 (方便测试).
func (r *Registry) Register(t Tool) {
	if t == nil {
		return
	}
	name := t.Schema().Function.Name
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[name] = t
}

// Get 按名称查找工具. 返回 false 表示未注册.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.items[name]
	return t, ok
}

// Names 返回所有工具名 (字典序), 便于展示.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.items))
	for k := range r.items {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Schemas 返回所有工具的 schema, 直接用于 ChatRequest.Tools.
func (r *Registry) Schemas() []llm.ToolSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.items))
	for k := range r.items {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]llm.ToolSchema, 0, len(names))
	for _, n := range names {
		out = append(out, r.items[n].Schema())
	}
	return out
}

// Schema 是构造 llm.ToolSchema 的辅助函数.
//
// params 是一个用来描述 JSON Schema 的 map, 直接走 json.Marshal.
// 返回值可以塞进 llm.ChatRequest.Tools.
func Schema(name, description string, params map[string]any) llm.ToolSchema {
	raw, err := json.Marshal(params)
	if err != nil {
		raw = []byte(`{"type":"object","properties":{}}`)
	}
	return llm.ToolSchema{
		Type: "function",
		Function: llm.FunctionSchema{
			Name:        name,
			Description: description,
			Parameters:  raw,
		},
	}
}

// ParseArgs 是 Tool 实现的常用 helper: 把 JSON args 反序列化到任意结构.
// 当 args 为空时返回 nil 错误, 调用方仍要自行决定缺省值.
func ParseArgs(args json.RawMessage, dst any) error {
	if len(args) == 0 || string(args) == "null" {
		return nil
	}
	if err := json.Unmarshal(args, dst); err != nil {
		return fmt.Errorf("parse args: %w", err)
	}
	return nil
}

// ErrUnknownTool 在 Registry 找不到名称时返回.
var ErrUnknownTool = errors.New("unknown tool")
