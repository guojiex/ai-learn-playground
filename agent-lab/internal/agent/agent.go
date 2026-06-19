// Package agent 提供 agent 循环的统一抽象.
//
// 本包包含两种模式:
//   - native: 依赖 OpenAI 兼容 server 的原生 function-calling (M2, tooling.go).
//   - react:  自写 JSON 协议的 Thought-Action-Observation 循环 (M3, react.go).
//
// 两种模式共用同一个 Agent 接口, 方便 CLI/Web 切换比较.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
)

// ErrMaxSteps 表示 agent 循环达到上限但仍未收敛.
var ErrMaxSteps = errors.New("max steps reached")

// StepKind 区分一个 step 的子阶段, 用于 UI 着色展示.
type StepKind string

const (
	StepThought      StepKind = "thought"
	StepAction       StepKind = "action"
	StepObservation  StepKind = "observation"
	StepFinal        StepKind = "final"
	StepParseRetry   StepKind = "parse_retry"
	StepParseDegrade StepKind = "parse_degrade"
)

// Step 描述 agent 循环的一步, 用于 UI / trace / JSON 导出.
type Step struct {
	StepIndex   int       `json:"step"`
	Kind        StepKind  `json:"kind"`
	Thought     string    `json:"thought,omitempty"`
	ActionName  string    `json:"action_name,omitempty"`
	ActionArgs  string    `json:"action_args,omitempty"`
	Observation string    `json:"observation,omitempty"`
	Error       string    `json:"error,omitempty"`
	ElapsedMS   int64     `json:"elapsed_ms"`
	StartedAt   time.Time `json:"started_at"`
}

// RunResult 是一次 agent Run 的完整输出.
type RunResult struct {
	Final   string        `json:"final"`
	Steps   []Step        `json:"steps"`
	Mode    string        `json:"mode"` // "native" | "react"
	Elapsed time.Duration `json:"elapsed"`
	Usage   llm.Usage     `json:"usage"`
}

// Agent 是 agent 循环的统一抽象, NativeAgent 与 ReActAgent 都实现它.
type Agent interface {
	// Run 执行一次完整的 agent 循环, 以 userMsg 为起点驱动到 final 或上限.
	Run(ctx context.Context, userMsg string) (RunResult, error)
	// Mode 返回当前 agent 的模式标识.
	Mode() string
}

// Options 是两种模式都需要的参数.
type Options struct {
	SystemPrompt string
	Model        string
	Temperature  float32
	MaxTokens    int
	MaxSteps     int
}

func (o Options) normalized() Options {
	if o.MaxSteps <= 0 {
		o.MaxSteps = 8
	}
	if o.Temperature <= 0 {
		o.Temperature = 0.4
	}
	if o.MaxTokens <= 0 {
		o.MaxTokens = 512
	}
	if o.SystemPrompt == "" {
		o.SystemPrompt = "你是一个电商文案助理."
	}
	return o
}

// marshalCompact 把任意对象压缩成 JSON 字符串, 空输入返回空串.
func marshalCompact(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
