package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// TaskStatus 是 DAG 节点的执行状态, 对应 UI 颜色.
type TaskStatus string

const (
	TaskPending TaskStatus = "pending" // 灰: 等待依赖完成
	TaskRunning TaskStatus = "running" // 蓝: 正在执行
	TaskOK      TaskStatus = "ok"      // 绿: 成功
	TaskFail    TaskStatus = "fail"    // 红: 失败
	TaskReplan  TaskStatus = "replan"  // 橙: 失败后等待重规划
	TaskSkipped TaskStatus = "skipped" // 灰虚: 因上游失败而跳过
)

// Task 是 Plan 中的一个子任务节点.
//
// 两种执行方式:
//   - tool: 调用 tools.Registry 里的工具, args 是 JSON 参数.
//   - agent: 调用 LLM 生成文本 (如 "writer" / "composer"), prompt 是给 LLM 的指令.
type Task struct {
	ID      string          `json:"id"`      // 唯一标识, 如 "t1"
	Name    string          `json:"name"`    // 人读名称, 如 "调研同品类爆款"
	Depends []string        `json:"depends"` // 依赖的 task ID 列表
	Tool    string          `json:"tool"`    // 工具名 (与 Agent 二选一)
	Args    json.RawMessage `json:"args"`    // 工具参数 JSON
	Agent   string          `json:"agent"`   // agent 角色名 (与 Tool 二选一), 如 "writer" / "composer"
	Prompt  string          `json:"prompt"`  // agent 任务的 LLM 指令模板
}

// TaskResult 记录一个子任务的执行结果.
type TaskResult struct {
	TaskID    string        `json:"task_id"`
	Status    TaskStatus    `json:"status"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	Elapsed   time.Duration `json:"elapsed"`
	Tokens    int           `json:"tokens"`
}

// Plan 是 Planner 产出的 DAG.
type Plan struct {
	Goal  string `json:"goal"`
	Tasks []Task `json:"tasks"`
}

// PlanRun 是一次完整的 Plan 执行过程, 含原始 plan + 所有 task 结果 + replan 记录.
type PlanRun struct {
	Goal        string         `json:"goal"`
	Plan        *Plan          `json:"plan"`
	Results     []TaskResult   `json:"results"`
	Replans     []ReplanRecord `json:"replans"`
	Status      string         `json:"status"` // "running" | "ok" | "fail"
	StartedAt   time.Time      `json:"started_at"`
	FinishedAt  time.Time      `json:"finished_at"`
	TotalTokens int            `json:"total_tokens"`
}

// ReplanRecord 记录一次重规划.
type ReplanRecord struct {
	Reason     string    `json:"reason"`
	FailedTask string    `json:"failed_task"`
	At         time.Time `json:"at"`
}

// TaskMap 把 Plan 的 Tasks 转成 id → Task 映射, 方便查依赖.
func (p *Plan) TaskMap() map[string]*Task {
	m := make(map[string]*Task, len(p.Tasks))
	for i := range p.Tasks {
		m[p.Tasks[i].ID] = &p.Tasks[i]
	}
	return m
}

// Validate 检查 Plan 的基本完整性: ID 唯一、依赖存在、无环.
func (p *Plan) Validate() error {
	if p == nil {
		return fmt.Errorf("plan is nil")
	}
	if len(p.Tasks) == 0 {
		return fmt.Errorf("plan has no tasks")
	}
	ids := make(map[string]bool)
	for i := range p.Tasks {
		t := &p.Tasks[i]
		if t.ID == "" {
			return fmt.Errorf("task %d has empty id", i)
		}
		if ids[t.ID] {
			return fmt.Errorf("duplicate task id: %s", t.ID)
		}
		ids[t.ID] = true
		if t.Tool == "" && t.Agent == "" {
			return fmt.Errorf("task %s has neither tool nor agent", t.ID)
		}
		if t.Tool != "" && t.Agent != "" {
			return fmt.Errorf("task %s has both tool and agent (pick one)", t.ID)
		}
		for _, dep := range t.Depends {
			if !ids[dep] {
				return fmt.Errorf("task %s depends on unknown task %s", t.ID, dep)
			}
		}
	}
	if cycle := p.detectCycle(); cycle != "" {
		return fmt.Errorf("cycle detected: %s", cycle)
	}
	return nil
}

// detectCycle 用 Kahn 拓扑排序检测环, 有环返回环上的节点序列.
func (p *Plan) detectCycle() string {
	inDeg := make(map[string]int)
	adj := make(map[string][]string)
	for i := range p.Tasks {
		t := &p.Tasks[i]
		inDeg[t.ID] = len(t.Depends)
		for _, dep := range t.Depends {
			adj[dep] = append(adj[dep], t.ID)
		}
	}
	var queue []string
	for id, d := range inDeg {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	processed := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		processed++
		for _, next := range adj[cur] {
			inDeg[next]--
			if inDeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if processed < len(p.Tasks) {
		// 有环: 收集 inDeg > 0 的节点.
		var cycleNodes []string
		for id, d := range inDeg {
			if d > 0 {
				cycleNodes = append(cycleNodes, id)
			}
		}
		sort.Strings(cycleNodes)
		return fmt.Sprintf("%v", cycleNodes)
	}
	return ""
}

// ReadyTasks 返回当前可执行的 task ID (依赖全部完成且自身未执行).
// done 是已完成的 task ID 集合.
func (p *Plan) ReadyTasks(done map[string]bool) []string {
	var ready []string
	for i := range p.Tasks {
		t := &p.Tasks[i]
		if done[t.ID] {
			continue
		}
		allDepsDone := true
		for _, dep := range t.Depends {
			if !done[dep] {
				allDepsDone = false
				break
			}
		}
		if allDepsDone {
			ready = append(ready, t.ID)
		}
	}
	sort.Strings(ready)
	return ready
}

// TopoLevels 把 DAG 分成若干 "层": 同层节点无依赖关系, 可并行执行.
// 用于 UI 的列布局可视化.
func (p *Plan) TopoLevels() [][]string {
	inDeg := make(map[string]int)
	adj := make(map[string][]string)
	for i := range p.Tasks {
		t := &p.Tasks[i]
		inDeg[t.ID] = len(t.Depends)
		for _, dep := range t.Depends {
			adj[dep] = append(adj[dep], t.ID)
		}
	}
	var levels [][]string
	var current []string
	for id, d := range inDeg {
		if d == 0 {
			current = append(current, id)
		}
	}
	for len(current) > 0 {
		sort.Strings(current)
		levels = append(levels, current)
		var next []string
		for _, cur := range current {
			for _, succ := range adj[cur] {
				inDeg[succ]--
				if inDeg[succ] == 0 {
					next = append(next, succ)
				}
			}
		}
		current = next
	}
	return levels
}

// float32Ptr / intPtr 是构造 *float32 / *int 的便捷函数, 供 ChatRequest 用.
func float32Ptr(v float32) *float32 { return &v }
func intPtr(v int) *int             { return &v }
