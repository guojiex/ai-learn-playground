// Package agent 提供 tool-calling 风格的多轮 agent 循环.
//
// M2 设计:
//   - 输入: ChatRequest 消息 + Registry.
//   - 主循环: 调 LLM -> 如果返回 tool_calls, 并行执行所有 tool, 每个结果以 role=tool 回填 -> 再调 LLM.
//   - 终止: finish_reason == "stop" 或者达到 maxSteps.
//   - 任何工具错误以 role=tool 回填给模型, 不打断循环.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// ToolCallRecord 描述一次 tool 调用的过程信息, 供 UI / trace 使用.
type ToolCallRecord struct {
	StepIndex  int             `json:"step"`
	CallID     string          `json:"call_id"`
	Name       string          `json:"name"`
	Args       json.RawMessage `json:"args"`
	Result     string          `json:"result,omitempty"`
	Err        string          `json:"error,omitempty"`
	StartedAt  time.Time       `json:"started_at"`
	DurationMS int64           `json:"duration_ms"`
}

// LoopOptions 是 Loop 的可调参数.
type LoopOptions struct {
	Model       string
	Temperature float32
	MaxTokens   int
	MaxSteps    int // 最多 LLM 调用次数 (含初次), 默认 8.
}

// Result 是 Loop 的最终输出.
type Result struct {
	FinalMessage llm.Message
	Steps        int
	ToolCalls    []ToolCallRecord
	Usage        llm.Usage
}

// Loop 执行 tool-calling 主循环. messages 入参会被复制, 不会原地修改.
func Loop(ctx context.Context, client llm.Client, reg *tools.Registry, messages []llm.Message, opts LoopOptions) (Result, error) {
	if client == nil {
		return Result{}, errors.New("llm client is nil")
	}
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = 8
	}
	if opts.Temperature <= 0 {
		opts.Temperature = 0.4
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 512
	}

	hist := make([]llm.Message, len(messages))
	copy(hist, messages)

	var schemas []llm.ToolSchema
	if reg != nil {
		schemas = reg.Schemas()
	}

	var (
		records []ToolCallRecord
		usage   llm.Usage
	)

	for step := 1; step <= opts.MaxSteps; step++ {
		req := llm.ChatRequest{
			Model:       opts.Model,
			Messages:    hist,
			Tools:       schemas,
			Temperature: &opts.Temperature,
			MaxTokens:   &opts.MaxTokens,
		}
		resp, err := client.Chat(ctx, req)
		if err != nil {
			return Result{Steps: step, ToolCalls: records, Usage: usage}, fmt.Errorf("step %d chat: %w", step, err)
		}
		usage.PromptTokens += resp.Usage.PromptTokens
		usage.CompletionTokens += resp.Usage.CompletionTokens
		usage.TotalTokens += resp.Usage.TotalTokens

		msg := resp.Message
		hist = append(hist, msg)

		// 没有 tool_calls 或 finish=stop 时收敛.
		if len(msg.ToolCalls) == 0 || resp.FinishReason == "stop" && len(msg.ToolCalls) == 0 {
			return Result{FinalMessage: msg, Steps: step, ToolCalls: records, Usage: usage}, nil
		}

		// 并行执行所有 tool_calls.
		newRecords, toolMsgs := runToolCalls(ctx, reg, step, msg.ToolCalls)
		records = append(records, newRecords...)
		hist = append(hist, toolMsgs...)
	}

	return Result{Steps: opts.MaxSteps, ToolCalls: records, Usage: usage}, fmt.Errorf("max steps %d reached", opts.MaxSteps)
}

// runToolCalls 并行执行 tool_calls, 返回顺序与入参一致.
func runToolCalls(ctx context.Context, reg *tools.Registry, step int, calls []llm.ToolCall) ([]ToolCallRecord, []llm.Message) {
	records := make([]ToolCallRecord, len(calls))
	msgs := make([]llm.Message, len(calls))

	var wg sync.WaitGroup
	for i, call := range calls {
		i, call := i, call
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := ToolCallRecord{
				StepIndex: step,
				CallID:    call.ID,
				Name:      call.Function.Name,
				Args:      json.RawMessage(call.Function.Arguments),
				StartedAt: time.Now(),
			}
			result, err := invokeOne(ctx, reg, call)
			rec.DurationMS = time.Since(rec.StartedAt).Milliseconds()
			content := result
			if err != nil {
				rec.Err = err.Error()
				// 错误也作为 tool message 回填给模型, 这样它能换参数重试.
				content = fmt.Sprintf(`{"error": %q}`, err.Error())
			} else {
				rec.Result = result
			}
			records[i] = rec
			msgs[i] = llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    content,
			}
		}()
	}
	wg.Wait()
	return records, msgs
}

func invokeOne(ctx context.Context, reg *tools.Registry, call llm.ToolCall) (string, error) {
	if reg == nil {
		return "", tools.ErrUnknownTool
	}
	t, ok := reg.Get(call.Function.Name)
	if !ok {
		return "", fmt.Errorf("%w: %s", tools.ErrUnknownTool, call.Function.Name)
	}
	return t.Invoke(ctx, json.RawMessage(call.Function.Arguments))
}
