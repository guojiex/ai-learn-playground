package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// ReActSystemPrompt 是 ReAct 协议的 system 提示, 明确告诉模型必须输出 JSON.
// 工具列表通过参数动态注入, 方便 CLI 和 Web 共用.
func ReActSystemPrompt(baseSystem, toolNames string) string {
	const tmpl = `你是一名电商文案助理, 必须按下面的 JSON 协议驱动 Thought-Action-Observation 循环.

规则:
1. 每轮你必须且只能输出一个 JSON 对象, 形式为:
     {"thought": "<你这一步的思考>", "action": {"name": "<工具名>", "args": {...}}}
   或者在有最终答案时输出:
     {"thought": "<总结>", "final": "<最终回答>"}
2. 可以调用的工具: __TOOLS__
// 3. 不要在 JSON 前后加解释性文字; 不要把 JSON 放在代码块里; 直接输出裸 JSON.
4. unknown tool / 参数错误 会得到 observation = {"error": "..."}; 请根据错误换参数或换工具.
5. 不要输出多段 JSON; 也不要在 JSON 里写 "..." 占位, 必须给出真实值.
6. 当你认为已经收集到足够信息就直接输出 "final" 字段并结束.

用户原始角色设定:
__BASE__`
	out := strings.Replace(tmpl, "__TOOLS__", toolNames, 1)
	out = strings.Replace(out, "__BASE__", baseSystem, 1)
	return out
}

// ReActAgent 是自写 JSON 协议的 agent 实现 (M3), 不依赖原生 function-calling.
type ReActAgent struct {
	Client llm.Client
	Reg    *tools.Registry
	Opts   Options
}

// NewReActAgent 创建一个 ReActAgent.
func NewReActAgent(client llm.Client, reg *tools.Registry, opts Options) *ReActAgent {
	return &ReActAgent{Client: client, Reg: reg, Opts: opts.normalized()}
}

// Mode 实现 Agent 接口.
func (a *ReActAgent) Mode() string { return "react" }

// Run 实现 Agent 接口: Thought-Action-Observation 循环.
func (a *ReActAgent) Run(ctx context.Context, userMsg string) (RunResult, error) {
	start := time.Now()
	if a.Client == nil {
		return RunResult{Mode: a.Mode()}, fmt.Errorf("llm client is nil")
	}

	toolNames := "无 (直接输出 final)"
	if a.Reg != nil {
		if names := a.Reg.Names(); len(names) > 0 {
			toolNames = strings.Join(names, ", ")
		}
	}
	systemPrompt := ReActSystemPrompt(a.Opts.SystemPrompt, toolNames)

	hist := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: userMsg},
	}

	var (
		steps      []Step
		usage      llm.Usage
		parseFails int
	)

	for stepIdx := 1; stepIdx <= a.Opts.MaxSteps; stepIdx++ {
		stepStart := time.Now()

		resp, err := a.Client.Chat(ctx, llm.ChatRequest{
			Model:       a.Opts.Model,
			Messages:    hist,
			Temperature: &a.Opts.Temperature,
			MaxTokens:   &a.Opts.MaxTokens,
			Stop:        []string{"```"},
		})
		if err != nil {
			return RunResult{Mode: a.Mode(), Steps: steps, Usage: usage, Elapsed: time.Since(start)},
				fmt.Errorf("step %d chat: %w", stepIdx, err)
		}
		usage.PromptTokens += resp.Usage.PromptTokens
		usage.CompletionTokens += resp.Usage.CompletionTokens
		usage.TotalTokens += resp.Usage.TotalTokens

		rawModel := resp.Message.Content
		parsed, parseErr := ParseReAct(rawModel)

		// 解析失败兜底: 最多给一次重发机会, 再失败就把原文当 final 降级.
		if parseErr != nil {
			parseFails++
			if parseFails == 1 {
				// 第一次解析失败: 把格式要求作为用户消息追加, 让模型重发.
				retryHint := fmt.Sprintf("你的输出不符合 JSON 协议 (错误: %s), 请严格按下面格式只输出一个 JSON 对象:\n{\"thought\":\"...\",\"action\":{\"name\":\"...\",\"args\":{...}}}  或  {\"thought\":\"...\",\"final\":\"...\"}", parseErr.Error())
				hist = append(hist,
					resp.Message,
					llm.Message{Role: llm.RoleUser, Content: retryHint},
				)
				steps = append(steps, Step{
					StepIndex: stepIdx,
					Kind:      StepParseRetry,
					Thought:   rawModel,
					Error:     parseErr.Error(),
					ElapsedMS: time.Since(stepStart).Milliseconds(),
					StartedAt: stepStart,
				})
				continue
			}
			// 第二次及以后解析失败: 降级, 把原文作为 final.
			steps = append(steps, Step{
				StepIndex: stepIdx,
				Kind:      StepParseDegrade,
				Thought:   rawModel,
				Error:     parseErr.Error(),
				ElapsedMS: time.Since(stepStart).Milliseconds(),
				StartedAt: stepStart,
			})
			return RunResult{
				Mode:    a.Mode(),
				Final:   strings.TrimSpace(rawModel),
				Steps:   steps,
				Usage:   usage,
				Elapsed: time.Since(start),
			}, nil
		}

		// 解析成功, 开始思考与执行.
		hist = append(hist, resp.Message)

		// 1) final 字段: 直接收敛.
		if strings.TrimSpace(parsed.Final) != "" && parsed.Action == nil {
			steps = append(steps, Step{
				StepIndex: stepIdx,
				Kind:      StepFinal,
				Thought:   parsed.Thought,
				ElapsedMS: time.Since(stepStart).Milliseconds(),
				StartedAt: stepStart,
			})
			return RunResult{
				Mode:    a.Mode(),
				Final:   strings.TrimSpace(parsed.Final),
				Steps:   steps,
				Usage:   usage,
				Elapsed: time.Since(start),
			}, nil
		}

		// 2) action: 调用工具, 把结果以 observation 形式追加.
		if parsed.Action != nil {
			actionStep := Step{
				StepIndex:  stepIdx,
				Kind:       StepAction,
				Thought:    parsed.Thought,
				ActionName: parsed.Action.Name,
				ActionArgs: marshalCompact(parsed.Action.Args),
				StartedAt:  stepStart,
			}
			obsText, obsErr := a.invokeTool(ctx, parsed.Action)
			actionStep.ElapsedMS = time.Since(stepStart).Milliseconds()
			if obsErr != nil {
				actionStep.Observation = fmt.Sprintf(`{"error": %q}`, obsErr.Error())
			} else {
				actionStep.Observation = obsText
			}
			steps = append(steps, actionStep)

			hist = append(hist, llm.Message{
				Role:    llm.RoleUser,
				Content: "observation(" + parsed.Action.Name + "): " + actionStep.Observation,
			})
			continue
		}

		// 3) 既无 action 也无 final: 当作解析异常, 给一次重发机会.
		parseFails++
		if parseFails <= 1 {
			hist = append(hist, llm.Message{Role: llm.RoleUser, Content: "你的输出缺少 action 和 final, 请按协议重新输出."})
			steps = append(steps, Step{
				StepIndex: stepIdx,
				Kind:      StepParseRetry,
				Thought:   rawModel,
				Error:     "missing action/final",
				ElapsedMS: time.Since(stepStart).Milliseconds(),
				StartedAt: stepStart,
			})
			continue
		}
		// 多次异常: 降级.
		steps = append(steps, Step{
			StepIndex: stepIdx,
			Kind:      StepParseDegrade,
			Thought:   rawModel,
			ElapsedMS: time.Since(stepStart).Milliseconds(),
			StartedAt: stepStart,
		})
		return RunResult{
			Mode:    a.Mode(),
			Final:   strings.TrimSpace(rawModel),
			Steps:   steps,
			Usage:   usage,
			Elapsed: time.Since(start),
		}, nil
	}

	// 达到最大步数仍未收敛.
	return RunResult{Mode: a.Mode(), Steps: steps, Usage: usage, Elapsed: time.Since(start)},
		ErrMaxSteps
}

// invokeTool 按 ReActParsed.Action 调用 registry 中的工具, 返回 observation 文本.
func (a *ReActAgent) invokeTool(ctx context.Context, act *ReActAction) (string, error) {
	if a.Reg == nil {
		return "", fmt.Errorf("no tools registered")
	}
	t, ok := a.Reg.Get(act.Name)
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", act.Name)
	}
	args := act.Args
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	return t.Invoke(ctx, args)
}
