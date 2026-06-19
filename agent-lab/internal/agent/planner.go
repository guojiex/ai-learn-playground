package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// Planner 用一次 LLM 调用把 "目标" 变成结构化 Plan (DAG).
//
// M6 核心思想: "先整体规划, 再分步执行". 与 ReAct 的 "走一步看一步" 互补.
// 复杂任务 (多步骤 / 有依赖) 用 Planner, 单点任务用 ReAct.
type Planner struct {
	client   llm.Client
	reg      *tools.Registry
	model    string
	maxRetry int
}

// NewPlanner 构造一个 Planner. reg 用于把可用工具名注入 prompt.
func NewPlanner(client llm.Client, reg *tools.Registry, model string) *Planner {
	return &Planner{
		client:   client,
		reg:      reg,
		model:    model,
		maxRetry: 2,
	}
}

// Plan 生成一个 Plan. 解析失败时最多重试 maxRetry 次, 每次把错误反馈给 LLM.
func (p *Planner) Plan(ctx context.Context, goal string) (*Plan, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("planner not configured")
	}
	toolList := "无"
	if p.reg != nil {
		if names := p.reg.Names(); len(names) > 0 {
			toolList = strings.Join(names, ", ")
		}
	}
	systemPrompt := plannerSystemPrompt(toolList)
	var lastErr error
	for attempt := 0; attempt <= p.maxRetry; attempt++ {
		userMsg := goal
		if attempt > 0 && lastErr != nil {
			userMsg = fmt.Sprintf("你上次的输出解析失败 (%s), 请重新生成一个合法的 Plan JSON.\n\n原始目标: %s", lastErr.Error(), goal)
		}
		resp, err := p.client.Chat(ctx, llm.ChatRequest{
			Model: p.model,
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: systemPrompt},
				{Role: llm.RoleUser, Content: userMsg},
			},
			Temperature: float32Ptr(0.3),
			MaxTokens:   intPtr(1024),
		})
		if err != nil {
			return nil, fmt.Errorf("planner chat: %w", err)
		}
		plan, err := parsePlan(resp.Message.Content)
		if err != nil {
			lastErr = err
			continue
		}
		if err := plan.Validate(); err != nil {
			lastErr = err
			continue
		}
		plan.Goal = goal
		return plan, nil
	}
	return nil, fmt.Errorf("planner failed after %d retries: %w", p.maxRetry, lastErr)
}

// Replan 在子任务失败后, 把 "进展 + 失败原因" 喂回 Planner, 生成新计划.
func (p *Planner) Replan(ctx context.Context, goal string, original *Plan, failedTaskID, failReason string, completedOutputs map[string]string) (*Plan, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("planner not configured")
	}
	toolList := "无"
	if p.reg != nil {
		if names := p.reg.Names(); len(names) > 0 {
			toolList = strings.Join(names, ", ")
		}
	}
	var progressSB strings.Builder
	for _, t := range original.Tasks {
		if out, ok := completedOutputs[t.ID]; ok {
			fmt.Fprintf(&progressSB, "- %s (%s): 已完成, 输出摘要: %s\n", t.ID, t.Name, truncateForError(out, 100))
		}
	}
	systemPrompt := replannerSystemPrompt(toolList)
	userMsg := fmt.Sprintf(`原始目标: %s

原计划失败:
- 失败任务: %s
- 失败原因: %s

已完成的任务及输出:
%s

请生成一个修订后的 Plan JSON, 保留已完成的任务, 修正失败的任务及后续依赖.`, goal, failedTaskID, failReason, progressSB.String())

	resp, err := p.client.Chat(ctx, llm.ChatRequest{
		Model: p.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: userMsg},
		},
		Temperature: float32Ptr(0.3),
		MaxTokens:   intPtr(1024),
	})
	if err != nil {
		return nil, fmt.Errorf("replanner chat: %w", err)
	}
	plan, err := parsePlan(resp.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("replanner parse: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("replanner validate: %w", err)
	}
	plan.Goal = goal
	return plan, nil
}

// parsePlan 从 LLM 原始文本中提取 Plan JSON, 复用 ReAct 的容错提取逻辑.
func parsePlan(raw string) (*Plan, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, fmt.Errorf("empty planner output")
	}
	candidates := make([]string, 0, 4)
	if strings.HasPrefix(text, "{") {
		candidates = append(candidates, text)
	}
	if s, ok := extractFenced(text, "```json", "```"); ok {
		candidates = append(candidates, s)
	}
	if s, ok := extractFenced(text, "```", "```"); ok {
		candidates = append(candidates, s)
	}
	if brace, ok := extractFirstBracePair(text); ok {
		candidates = append(candidates, brace)
	}
	for _, c := range candidates {
		var plan Plan
		if err := json.Unmarshal([]byte(c), &plan); err == nil {
			return &plan, nil
		}
	}
	return nil, fmt.Errorf("cannot parse plan JSON: %s", truncateForError(text, 120))
}

func plannerSystemPrompt(toolList string) string {
	return fmt.Sprintf(`你是一个任务规划器 (Planner). 你的职责是把用户的目标拆解成一个 DAG (有向无环图) 计划.

输出格式: 严格输出一个 JSON 对象, 不要加任何解释文字, 不要用代码块包裹.

JSON 协议:
{
  "goal": "<用户的目标>",
  "tasks": [
    {
      "id": "t1",
      "name": "<人读名称>",
      "depends": [],
      "tool": "<工具名>",
      "args": {<工具参数 JSON>}
    },
    {
      "id": "t2",
      "name": "<人读名称>",
      "depends": ["t1"],
      "agent": "writer",
      "prompt": "<给 LLM 的指令, 可引用 {t1.output} 表示上游任务输出>"
    }
  ]
}

规则:
1. 每个 task 必须有 id (t1, t2, ...) 和 name.
2. tool 和 agent 二选一: tool 调用工具, agent 调 LLM 生成文本.
3. depends 列出依赖的 task id, 依赖必须已完成才能执行.
4. agent 任务的 prompt 中可用 {<task_id>.output} 引用上游任务输出.
5. 可用工具: %s
6. 典型流程: 调研(kb_search) → 查商品(product_lookup) → 写文案(writer) → 合规检查(platform_lint) → 组合输出(composer).
7. 任务数量 3-6 个为宜, 不要过度拆分.`, toolList)
}

func replannerSystemPrompt(toolList string) string {
	return fmt.Sprintf(`你是一个任务规划器 (Planner). 之前的计划执行失败, 你需要根据失败原因生成修订后的计划.

输出格式: 严格输出一个 JSON 对象 (Plan 协议), 不要加任何解释文字.

规则:
1. 保留已完成的任务 (标记为已完成, 不需要重新执行).
2. 修正失败的任务 (调整 args 或 prompt).
3. 可用工具: %s
4. agent 任务的 prompt 中可用 {<task_id>.output} 引用上游任务输出.`, toolList)
}
