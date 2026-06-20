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

// MultiAgent 协调 4 个角色 (Researcher → Writer → Critic + Compliance) 的多轮协作 (M7).
//
// 终止条件:
//  1. Critic + Compliance 都 approve.
//  2. 达到 maxRounds.
//  3. 循环防御: 连续 staleRounds 轮 draft 相似度 > threshold (无改进).
type MultiAgent struct {
	client         llm.Client
	reg            *tools.Registry
	model          string
	bus            *MessageBus
	maxRounds      int
	staleRounds    int
	staleThreshold float64
	agents         map[RoleName]*RoleAgent
}

// MultiEvent 是推送给 UI 的 SSE 事件.
type MultiEvent struct {
	Type        string   `json:"type"` // "round_start" | "agent_done" | "round_end" | "done" | "fail"
	Round       int      `json:"round,omitempty"`
	Agent       RoleName `json:"agent,omitempty"`
	Output      string   `json:"output,omitempty"`
	Approved    bool     `json:"approved,omitempty"`
	Issues      []string `json:"issues,omitempty"`
	Error       string   `json:"error,omitempty"`
	Feedback    string   `json:"feedback,omitempty"`
	Tokens      int      `json:"tokens,omitempty"`
	TotalTokens int      `json:"total_tokens,omitempty"`
	Elapsed     string   `json:"elapsed,omitempty"`
}

// MultiRunResult 是一次多 agent 协作的完整结果.
type MultiRunResult struct {
	RunID       string           `json:"run_id"`
	Goal        string           `json:"goal"`
	Rounds      int              `json:"rounds"`
	Status      string           `json:"status"` // "ok" | "max_rounds" | "stale"
	FinalDraft  string           `json:"final_draft"`
	TotalTokens int              `json:"total_tokens"`
	Results     []RoleStepResult `json:"results"`
	StartedAt   time.Time        `json:"started_at"`
	FinishedAt  time.Time        `json:"finished_at"`
}

// NewMultiAgent 构造一个多 agent 协调器.
func NewMultiAgent(client llm.Client, reg *tools.Registry, model string, bus *MessageBus) *MultiAgent {
	agents := make(map[RoleName]*RoleAgent)
	for _, role := range AllRoles() {
		agents[role.Name] = NewRoleAgent(role, client, reg, model)
	}
	return &MultiAgent{
		client:         client,
		reg:            reg,
		model:          model,
		bus:            bus,
		maxRounds:      4,
		staleRounds:    2,
		staleThreshold: 0.9,
		agents:         agents,
	}
}

// SetMaxRounds 覆盖默认最大轮次.
func (m *MultiAgent) SetMaxRounds(n int) {
	if n > 0 {
		m.maxRounds = n
	}
}

// Bus 返回关联的消息总线.
func (m *MultiAgent) Bus() *MessageBus {
	return m.bus
}

// Run 执行多 agent 协作, 通过 events channel 推送进度.
func (m *MultiAgent) Run(ctx context.Context, goal string, events chan<- MultiEvent) (*MultiRunResult, error) {
	start := time.Now()
	result := &MultiRunResult{
		RunID:     m.bus.RunID(),
		Goal:      goal,
		StartedAt: start,
		Status:    "running",
	}

	feedback := ""
	totalTokens := 0
	lastDraft := ""
	staleCount := 0

	for round := 1; round <= m.maxRounds; round++ {
		if events != nil {
			events <- MultiEvent{Type: "round_start", Round: round}
		}

		// 1. Researcher: 收集商品信息与卖点.
		researchInput := goal
		if feedback != "" {
			researchInput += "\n\n上一轮反馈: " + feedback
		}
		m.bus.Post(ctx, round, "user", RoleCoordinator, RoleResearcher, researchInput, "")
		researchOut, tokens, err := m.agents[RoleResearcher].Step(ctx, researchInput, true)
		totalTokens += tokens
		resResearch := RoleStepResult{Agent: RoleResearcher, Round: round, Output: researchOut, Tokens: tokens}
		if err != nil {
			resResearch.Error = err.Error()
			result.Results = append(result.Results, resResearch)
			if events != nil {
				events <- MultiEvent{Type: "agent_done", Round: round, Agent: RoleResearcher, Error: err.Error()}
			}
			result.Status = "fail"
			result.FinishedAt = time.Now()
			result.TotalTokens = totalTokens
			if events != nil {
				events <- MultiEvent{Type: "fail", Error: err.Error()}
			}
			return result, err
		}
		m.bus.Post(ctx, round, "assistant", RoleResearcher, RoleCoordinator, researchOut, "")
		result.Results = append(result.Results, resResearch)
		if events != nil {
			events <- MultiEvent{Type: "agent_done", Round: round, Agent: RoleResearcher, Output: truncateForError(researchOut, 200), Tokens: tokens}
		}

		// 2. Writer: 根据调研结果写文案.
		writerInput := fmt.Sprintf("目标: %s\n\n调研结果:\n%s", goal, researchOut)
		if feedback != "" {
			writerInput += "\n\n上一轮反馈 (请据此修改):\n" + feedback
		}
		m.bus.Post(ctx, round, "user", RoleCoordinator, RoleWriter, writerInput, "")
		writerOut, tokens2, err := m.agents[RoleWriter].Step(ctx, writerInput, false)
		totalTokens += tokens2
		resWriter := RoleStepResult{Agent: RoleWriter, Round: round, Output: writerOut, Tokens: tokens2}
		if err != nil {
			resWriter.Error = err.Error()
			result.Results = append(result.Results, resWriter)
			result.Status = "fail"
			result.FinishedAt = time.Now()
			result.TotalTokens = totalTokens
			if events != nil {
				events <- MultiEvent{Type: "fail", Error: err.Error()}
			}
			return result, err
		}
		m.bus.Post(ctx, round, "assistant", RoleWriter, RoleCoordinator, writerOut, "")
		result.Results = append(result.Results, resWriter)
		result.FinalDraft = writerOut
		if events != nil {
			events <- MultiEvent{Type: "agent_done", Round: round, Agent: RoleWriter, Output: truncateForError(writerOut, 200), Tokens: tokens2}
		}

		// 3. Critic: 评审文案.
		criticInput := fmt.Sprintf("文案:\n%s", writerOut)
		m.bus.Post(ctx, round, "user", RoleCoordinator, RoleCritic, criticInput, "")
		criticOut, tokens3, err := m.agents[RoleCritic].Step(ctx, criticInput, true)
		totalTokens += tokens3
		resCritic := RoleStepResult{Agent: RoleCritic, Round: round, Output: criticOut, Tokens: tokens3}
		criticApproved := false
		var criticIssues []string
		if err != nil {
			resCritic.Error = err.Error()
		} else {
			m.bus.Post(ctx, round, "assistant", RoleCritic, RoleCoordinator, criticOut, "")
			if parsed, perr := ParseRoleJSON(criticOut); perr == nil {
				criticApproved = IsApproved(parsed)
				criticIssues = GetIssues(parsed, "issues")
			}
		}
		resCritic.Approved = criticApproved
		resCritic.Issues = criticIssues
		result.Results = append(result.Results, resCritic)
		if events != nil {
			events <- MultiEvent{Type: "agent_done", Round: round, Agent: RoleCritic, Output: truncateForError(criticOut, 200), Approved: criticApproved, Issues: criticIssues, Tokens: tokens3}
		}

		// 4. Compliance: 合规检查.
		compInput := fmt.Sprintf("文案:\n%s", writerOut)
		m.bus.Post(ctx, round, "user", RoleCoordinator, RoleCompliance, compInput, "")
		compOut, tokens4, err := m.agents[RoleCompliance].Step(ctx, compInput, true)
		totalTokens += tokens4
		resComp := RoleStepResult{Agent: RoleCompliance, Round: round, Output: compOut, Tokens: tokens4}
		compApproved := false
		var compIssues []string
		if err != nil {
			resComp.Error = err.Error()
		} else {
			m.bus.Post(ctx, round, "assistant", RoleCompliance, RoleCoordinator, compOut, "")
			if parsed, perr := ParseRoleJSON(compOut); perr == nil {
				compApproved = IsApproved(parsed)
				compIssues = GetIssues(parsed, "violations")
			}
		}
		resComp.Approved = compApproved
		resComp.Issues = compIssues
		result.Results = append(result.Results, resComp)
		if events != nil {
			events <- MultiEvent{Type: "agent_done", Round: round, Agent: RoleCompliance, Output: truncateForError(compOut, 200), Approved: compApproved, Issues: compIssues, Tokens: tokens4}
		}

		// 5. 判断是否通过.
		if criticApproved && compApproved {
			result.Status = "ok"
			result.Rounds = round
			result.TotalTokens = totalTokens
			result.FinishedAt = time.Now()
			if events != nil {
				events <- MultiEvent{Type: "done", TotalTokens: totalTokens, Elapsed: time.Since(start).Round(time.Millisecond).String()}
			}
			return result, nil
		}

		// 6. 合并反馈给下一轮.
		var allIssues []string
		allIssues = append(allIssues, criticIssues...)
		allIssues = append(allIssues, compIssues...)
		feedback = strings.Join(allIssues, "; ")

		if events != nil {
			events <- MultiEvent{Type: "round_end", Round: round, Feedback: feedback}
		}

		// 7. 循环防御: draft 相似度检测.
		if lastDraft != "" {
			sim := textSimilarity(lastDraft, writerOut)
			if sim > m.staleThreshold {
				staleCount++
				if staleCount >= m.staleRounds {
					result.Status = "stale"
					result.Rounds = round
					result.TotalTokens = totalTokens
					result.FinishedAt = time.Now()
					if events != nil {
						events <- MultiEvent{Type: "done", Error: "stale: no improvement for " + fmt.Sprintf("%d", staleCount) + " rounds", TotalTokens: totalTokens}
					}
					return result, nil
				}
			} else {
				staleCount = 0
			}
		}
		lastDraft = writerOut
	}

	result.Status = "max_rounds"
	result.Rounds = m.maxRounds
	result.TotalTokens = totalTokens
	result.FinishedAt = time.Now()
	if events != nil {
		events <- MultiEvent{Type: "done", Error: "max rounds reached", TotalTokens: totalTokens}
	}
	return result, nil
}

// textSimilarity 用字符 bigram Jaccard 相似度粗略估计两段文本的相似度.
// 返回 [0, 1], 1 表示完全相同.
func textSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := bigramSet(a)
	setB := bigramSet(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	intersection := 0
	for k := range setA {
		if setB[k] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func bigramSet(s string) map[string]bool {
	runes := []rune(s)
	if len(runes) < 2 {
		return map[string]bool{}
	}
	set := make(map[string]bool, len(runes)-1)
	for i := 0; i < len(runes)-1; i++ {
		set[string(runes[i])+string(runes[i+1])] = true
	}
	return set
}

// MarshalResult 把 MultiRunResult 序列化成 JSON.
func MarshalResult(r *MultiRunResult) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
