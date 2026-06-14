package agent

import (
	"context"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// NativeAgent 包装 M2 的 Loop(), 以原生 function-calling 驱动 agent 循环.
// 实现 Agent 接口, 便于与 ReActAgent 做对照实验.
type NativeAgent struct {
	Client llm.Client
	Reg    *tools.Registry
	Opts   Options
}

// NewNativeAgent 创建一个 NativeAgent.
func NewNativeAgent(client llm.Client, reg *tools.Registry, opts Options) *NativeAgent {
	return &NativeAgent{Client: client, Reg: reg, Opts: opts.normalized()}
}

// Mode 实现 Agent 接口.
func (a *NativeAgent) Mode() string { return "native" }

// Run 实现 Agent 接口: 调用 Loop() 并把它的记录翻译成 RunResult.
func (a *NativeAgent) Run(ctx context.Context, userMsg string) (RunResult, error) {
	start := time.Now()
	res, err := Loop(ctx, a.Client, a.Reg, []llm.Message{
		{Role: llm.RoleSystem, Content: a.Opts.SystemPrompt},
		{Role: llm.RoleUser, Content: userMsg},
	}, LoopOptions{
		Model:       a.Opts.Model,
		Temperature: a.Opts.Temperature,
		MaxTokens:   a.Opts.MaxTokens,
		MaxSteps:    a.Opts.MaxSteps,
	})

	steps := make([]Step, 0, len(res.ToolCalls))
	for _, tc := range res.ToolCalls {
		s := Step{
			StepIndex:  tc.StepIndex,
			Kind:       StepAction,
			ActionName: tc.Name,
			ActionArgs: marshalCompact(tc.Args),
			StartedAt:  tc.StartedAt,
			ElapsedMS:  tc.DurationMS,
		}
		if tc.Err != "" {
			s.Observation = `{"error": "` + tc.Err + `"}`
			s.Error = tc.Err
		} else {
			s.Observation = tc.Result
		}
		steps = append(steps, s)
	}

	return RunResult{
		Mode:    a.Mode(),
		Final:   res.FinalMessage.Content,
		Steps:   steps,
		Usage:   res.Usage,
		Elapsed: time.Since(start),
	}, err
}
