package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ModelEntry 描述一个可路由到的本地模型 (M10).
type ModelEntry struct {
	Name    string   `json:"name"`     // 模型名, 例如 qwen2.5-7b-instruct
	BaseURL string   `json:"base_url"` // OpenAI 兼容 endpoint
	Ctx     int      `json:"ctx"`      // 上下文窗口 (tokens)
	Tags    []string `json:"tags"`     // 能力标签, 例如 fast/title/reason/planner
	EstTPS  float64  `json:"est_tps"`  // 预估 tokens/sec (本地成本估算)
}

// Registry 是模型注册表 (M10).
type Registry struct {
	Profile string       `json:"profile"`
	Models  []ModelEntry `json:"models"`
}

// ByTag 按 tag 查找第一个匹配的模型.
func (r *Registry) ByTag(tag string) (*ModelEntry, bool) {
	for i := range r.Models {
		for _, t := range r.Models[i].Tags {
			if t == tag {
				return &r.Models[i], true
			}
		}
	}
	return nil, false
}

// ByName 按模型名查找.
func (r *Registry) ByName(name string) (*ModelEntry, bool) {
	for i := range r.Models {
		if r.Models[i].Name == name {
			return &r.Models[i], true
		}
	}
	return nil, false
}

// Names 返回全部模型名.
func (r *Registry) Names() []string {
	out := make([]string, len(r.Models))
	for i, m := range r.Models {
		out[i] = m.Name
	}
	return out
}

// LoadRegistry 从 JSON 文件加载模型注册表.
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return &reg, nil
}

// DefaultRegistry 返回一个适合 fake-openai 的默认注册表 (3 个模型, 同一 base_url).
func DefaultRegistry(baseURL, profile string) *Registry {
	return &Registry{
		Profile: profile,
		Models: []ModelEntry{
			{
				Name:    "qwen2.5-3b-instruct",
				BaseURL: baseURL,
				Ctx:     8192,
				Tags:    []string{"fast", "title", "tag"},
				EstTPS:  80,
			},
			{
				Name:    "qwen2.5-7b-instruct",
				BaseURL: baseURL,
				Ctx:     8192,
				Tags:    []string{"default", "body", "critic"},
				EstTPS:  45,
			},
			{
				Name:    "qwen2.5-14b-instruct",
				BaseURL: baseURL,
				Ctx:     8192,
				Tags:    []string{"reason", "planner"},
				EstTPS:  25,
			},
		},
	}
}

// RouteRecord 记录一次路由决策 (供 UI 展示).
type RouteRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Task      string    `json:"task"`
	CtxTokens int       `json:"ctx_tokens"`
	Chosen    string    `json:"chosen"`    // 最终命中的模型名
	Fallbacks []string  `json:"fallbacks"` // 尝试过的 fallback 链
	Reason    string    `json:"reason"`    // 路由原因
	LatencyMs int64     `json:"latency_ms"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}
