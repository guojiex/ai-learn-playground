package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// Executor 按 DAG 依赖调度 Plan 中的子任务 (M6).
//
// 策略:
//   - 拓扑分层: 同层无依赖的节点并发执行 (受限并发度).
//   - 上下文裁剪: 每个 agent 子任务只看自己依赖的输出, 不看全局历史.
//   - Replan: 子任务失败时, 调 Planner.Replan 生成新计划, 最多 maxReplan 次.
type Executor struct {
	planner     *Planner
	client      llm.Client
	reg         *tools.Registry
	model       string
	maxReplan   int
	concurrency int
}

// NewExecutor 构造一个执行器.
func NewExecutor(planner *Planner, client llm.Client, reg *tools.Registry, model string) *Executor {
	return &Executor{
		planner:     planner,
		client:      client,
		reg:         reg,
		model:       model,
		maxReplan:   2,
		concurrency: 4,
	}
}

// ExecEvent 是执行过程中推送给 UI 的 SSE 事件.
type ExecEvent struct {
	Type     string        `json:"type"` // "task_start" | "task_done" | "task_fail" | "replan" | "plan_done" | "plan_fail"
	TaskID   string        `json:"task_id,omitempty"`
	TaskName string        `json:"task_name,omitempty"`
	Status   TaskStatus    `json:"status,omitempty"`
	Output   string        `json:"output,omitempty"`
	Error    string        `json:"error,omitempty"`
	Replan   *ReplanRecord `json:"replan,omitempty"`
	PlanRun  *PlanRun      `json:"plan_run,omitempty"`
	Elapsed  string        `json:"elapsed,omitempty"`
}

// Execute 执行一个 Plan, 通过 events channel 推送进度.
// events 为 nil 时不推送 (CLI 模式).
func (e *Executor) Execute(ctx context.Context, plan *Plan, events chan<- ExecEvent) (*PlanRun, error) {
	if e == nil {
		return nil, fmt.Errorf("executor not configured")
	}
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("invalid plan: %w", err)
	}

	run := &PlanRun{
		Goal:      plan.Goal,
		Plan:      plan,
		StartedAt: time.Now(),
		Status:    "running",
	}

	currentPlan := plan
	completedOutputs := make(map[string]string)

	for replanCount := 0; replanCount <= e.maxReplan; replanCount++ {
		results, failedTask, failErr := e.executeOnce(ctx, currentPlan, completedOutputs, events)
		run.Results = append(run.Results, results...)

		if failedTask == "" {
			// 全部成功.
			run.Status = "ok"
			run.FinishedAt = time.Now()
			run.TotalTokens = sumTokens(results)
			if events != nil {
				events <- ExecEvent{Type: "plan_done", PlanRun: run}
			}
			return run, nil
		}

		// 有失败: 尝试 replan.
		if replanCount >= e.maxReplan {
			run.Status = "fail"
			run.FinishedAt = time.Now()
			run.TotalTokens = sumTokens(results)
			if events != nil {
				events <- ExecEvent{Type: "plan_fail", Error: fmt.Sprintf("task %s failed: %s (max replan reached)", failedTask, failErr), PlanRun: run}
			}
			return run, fmt.Errorf("task %s failed: %s (max replan %d reached)", failedTask, failErr, e.maxReplan)
		}

		// Replan.
		rec := ReplanRecord{
			Reason:     failErr.Error(),
			FailedTask: failedTask,
			At:         time.Now(),
		}
		run.Replans = append(run.Replans, rec)
		if events != nil {
			events <- ExecEvent{Type: "replan", Replan: &rec}
		}
		newPlan, err := e.planner.Replan(ctx, plan.Goal, currentPlan, failedTask, failErr.Error(), completedOutputs)
		if err != nil {
			run.Status = "fail"
			run.FinishedAt = time.Now()
			if events != nil {
				events <- ExecEvent{Type: "plan_fail", Error: fmt.Sprintf("replan failed: %s", err), PlanRun: run}
			}
			return run, fmt.Errorf("replan failed: %w", err)
		}
		currentPlan = newPlan
		run.Plan = newPlan
	}

	run.Status = "fail"
	run.FinishedAt = time.Now()
	return run, fmt.Errorf("max replan reached")
}

// executeOnce 执行一轮 plan, 返回结果 + 第一个失败的 task (如果有).
func (e *Executor) executeOnce(ctx context.Context, plan *Plan, completedOutputs map[string]string, events chan<- ExecEvent) ([]TaskResult, string, error) {
	taskMap := plan.TaskMap()
	results := make([]TaskResult, 0, len(plan.Tasks))
	resultMap := make(map[string]*TaskResult)
	done := make(map[string]bool)

	// 标记已完成的 (replan 场景下复用).
	for id := range completedOutputs {
		if _, exists := taskMap[id]; exists {
			done[id] = true
		}
	}

	for {
		ready := plan.ReadyTasks(done)
		if len(ready) == 0 {
			break
		}

		// 并发执行 ready 任务 (受限并发度).
		sem := make(chan struct{}, e.concurrency)
		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstFailID string
		var firstFailErr error

		for _, taskID := range ready {
			task := taskMap[taskID]
			sem <- struct{}{}
			wg.Add(1)
			go func(t *Task) {
				defer wg.Done()
				defer func() { <-sem }()

				res := e.executeTask(ctx, t, completedOutputs)
				if events != nil {
					evType := "task_done"
					if res.Status == TaskFail {
						evType = "task_fail"
					}
					events <- ExecEvent{
						Type:     evType,
						TaskID:   t.ID,
						TaskName: t.Name,
						Status:   res.Status,
						Output:   truncateForError(res.Output, 200),
						Error:    res.Error,
						Elapsed:  fmt.Sprintf("%.1fs", res.Elapsed.Seconds()),
					}
				}

				mu.Lock()
				resultMap[t.ID] = &res
				if res.Status == TaskFail && firstFailID == "" {
					firstFailID = t.ID
					firstFailErr = fmt.Errorf("%s", res.Error)
				}
				mu.Unlock()
			}(task)
		}
		wg.Wait()

		// 收集本轮结果.
		for _, taskID := range ready {
			if res, ok := resultMap[taskID]; ok {
				results = append(results, *res)
				if res.Status == TaskOK {
					done[taskID] = true
					completedOutputs[taskID] = res.Output
				}
			}
		}

		// 有失败: 标记后续任务为 skipped, 返回.
		if firstFailID != "" {
			for i := range plan.Tasks {
				t := &plan.Tasks[i]
				if !done[t.ID] && t.ID != firstFailID {
					if _, hasResult := resultMap[t.ID]; !hasResult {
						skipped := TaskResult{
							TaskID: t.ID,
							Status: TaskSkipped,
						}
						results = append(results, skipped)
					}
				}
			}
			return results, firstFailID, firstFailErr
		}
	}

	return results, "", nil
}

// executeTask 执行单个 task (tool 或 agent).
func (e *Executor) executeTask(ctx context.Context, t *Task, completedOutputs map[string]string) TaskResult {
	start := time.Now()
	res := TaskResult{
		TaskID:    t.ID,
		Status:    TaskRunning,
		StartedAt: start,
	}
	defer func() {
		res.Elapsed = time.Since(start)
	}()

	if t.Tool != "" {
		output, err := e.execTool(ctx, t, completedOutputs)
		if err != nil {
			res.Status = TaskFail
			res.Error = err.Error()
			return res
		}
		res.Status = TaskOK
		res.Output = output
		return res
	}

	// agent task: 调 LLM.
	output, tokens, err := e.execAgent(ctx, t, completedOutputs)
	if err != nil {
		res.Status = TaskFail
		res.Error = err.Error()
		return res
	}
	res.Status = TaskOK
	res.Output = output
	res.Tokens = tokens
	return res
}

// execTool 调用 registry 中的工具.
func (e *Executor) execTool(ctx context.Context, t *Task, completedOutputs map[string]string) (string, error) {
	if e.reg == nil {
		return "", fmt.Errorf("no tools registry")
	}
	tool, ok := e.reg.Get(t.Tool)
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", t.Tool)
	}
	args := t.Args
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	// 替换 args 中的上游输出引用.
	args = substituteRefs(args, completedOutputs)
	return tool.Invoke(ctx, args)
}

// execAgent 调用 LLM 执行 agent 子任务 (writer / composer 等).
func (e *Executor) execAgent(ctx context.Context, t *Task, completedOutputs map[string]string) (string, int, error) {
	if e.client == nil {
		return "", 0, fmt.Errorf("no llm client")
	}
	prompt := substituteRefsStr(t.Prompt, completedOutputs)
	systemPrompt := fmt.Sprintf("你是电商文案团队中的 %s 角色. 根据指令完成任务.", t.Agent)
	if t.Agent == "" {
		systemPrompt = "你是电商文案助理. 根据指令完成任务."
		t.Agent = "assistant"
	}
	resp, err := e.client.Chat(ctx, llm.ChatRequest{
		Model: e.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
		Temperature: float32Ptr(0.6),
		MaxTokens:   intPtr(512),
	})
	if err != nil {
		return "", 0, fmt.Errorf("agent %s chat: %w", t.Agent, err)
	}
	return strings.TrimSpace(resp.Message.Content), resp.Usage.TotalTokens, nil
}

// substituteRefs 把 args JSON 中的 {t1.output} 替换为上游输出.
func substituteRefs(args json.RawMessage, outputs map[string]string) json.RawMessage {
	s := string(args)
	for id, out := range outputs {
		ref := fmt.Sprintf("{%s.output}", id)
		// JSON 字符串里要转义引号.
		escaped := jsonEscapeString(out)
		s = strings.ReplaceAll(s, ref, escaped)
	}
	return json.RawMessage(s)
}

// substituteRefsStr 把 prompt 字符串中的 {t1.output} 替换为上游输出.
func substituteRefsStr(prompt string, outputs map[string]string) string {
	s := prompt
	for id, out := range outputs {
		ref := fmt.Sprintf("{%s.output}", id)
		s = strings.ReplaceAll(s, ref, out)
	}
	return s
}

func completedOutputsSafe(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func jsonEscapeString(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

func sumTokens(results []TaskResult) int {
	total := 0
	for _, r := range results {
		total += r.Tokens
	}
	return total
}
