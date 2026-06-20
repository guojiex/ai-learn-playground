package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// Role 定义一个多 agent 角色的配置.
type Role struct {
	Name         RoleName
	SystemPrompt string
	ToolNames    []string
}

// RoleStepResult 是一个角色执行一步的输出.
type RoleStepResult struct {
	Agent    RoleName `json:"agent"`
	Round    int      `json:"round"`
	Output   string   `json:"output"`
	Tokens   int      `json:"tokens"`
	Approved bool     `json:"approved,omitempty"`
	Issues   []string `json:"issues,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// --- Researcher ---

func ResearcherRole() Role {
	return Role{
		Name: RoleResearcher,
		SystemPrompt: `你是电商文案团队的调研员 (Researcher). 职责: 收集商品信息与竞品卖点.
输出格式: JSON {"facts": ["卖点1", "卖点2", ...], "competitors": ["竞品1", ...], "summary": "一句话总结"}
只输出 JSON, 不要解释.`,
		ToolNames: []string{"product_lookup", "kb_search"},
	}
}

// --- Writer ---

func WriterRole() Role {
	return Role{
		Name: RoleWriter,
		SystemPrompt: `你是电商文案团队的撰稿人 (Writer). 职责: 根据调研结果撰写平台文案.
输出格式: JSON {"title": "标题", "body": "正文", "tags": ["#标签1", "#标签2"]}
只输出 JSON, 不要解释. 若有反馈, 请据此修改.`,
		ToolNames: nil,
	}
}

// --- Critic ---

func CriticRole() Role {
	return Role{
		Name: RoleCritic,
		SystemPrompt: `你是电商文案团队的评审 (Critic). 职责: 检查文案的吸引力与完整性.
输出格式: JSON {"approve": true/false, "issues": ["问题1", "问题2"]}
approve=true 表示通过; 否则列出问题供 Writer 修改.
只输出 JSON, 不要解释.`,
		ToolNames: []string{"slang_check"},
	}
}

// --- Compliance ---

func ComplianceRole() Role {
	return Role{
		Name: RoleCompliance,
		SystemPrompt: `你是电商文案团队的合规审查员 (Compliance). 职责: 检查文案是否违反平台规则 (违禁词/字数/格式).
输出格式: JSON {"approve": true/false, "violations": ["违规1", "违规2"]}
approve=true 表示合规; 否则列出违规项.
只输出 JSON, 不要解释.`,
		ToolNames: []string{"platform_lint"},
	}
}

// AllRoles 返回全部 4 个角色的定义.
func AllRoles() []Role {
	return []Role{ResearcherRole(), WriterRole(), CriticRole(), ComplianceRole()}
}

// RoleAgent 封装一个角色的执行逻辑: 调 LLM (可能带工具) 产出结构化输出.
type RoleAgent struct {
	role   Role
	client llm.Client
	reg    *tools.Registry
	model  string
}

// NewRoleAgent 构造一个角色 agent.
func NewRoleAgent(role Role, client llm.Client, reg *tools.Registry, model string) *RoleAgent {
	return &RoleAgent{role: role, client: client, reg: reg, model: model}
}

// Step 执行一步: 把 input 发给 LLM, 返回输出文本 + token 数.
// tools 为 true 时注入角色的工具子集 (用 native Loop 驱动).
func (a *RoleAgent) Step(ctx context.Context, input string, useTools bool) (string, int, error) {
	if a.client == nil {
		return "", 0, fmt.Errorf("llm client is nil")
	}

	if useTools && a.reg != nil && len(a.role.ToolNames) > 0 {
		return a.stepWithTools(ctx, input)
	}

	resp, err := a.client.Chat(ctx, llm.ChatRequest{
		Model: a.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: a.role.SystemPrompt},
			{Role: llm.RoleUser, Content: input},
		},
		Temperature: float32Ptr(0.5),
		MaxTokens:   intPtr(512),
	})
	if err != nil {
		return "", 0, fmt.Errorf("%s chat: %w", a.role.Name, err)
	}
	return strings.TrimSpace(resp.Message.Content), resp.Usage.TotalTokens, nil
}

// stepWithTools 用 native Loop 驱动带工具的角色.
func (a *RoleAgent) stepWithTools(ctx context.Context, input string) (string, int, error) {
	subReg := tools.NewRegistry()
	for _, name := range a.role.ToolNames {
		if t, ok := a.reg.Get(name); ok {
			subReg.Register(t)
		}
	}
	result, err := Loop(ctx, a.client, subReg, []llm.Message{
		{Role: llm.RoleSystem, Content: a.role.SystemPrompt},
		{Role: llm.RoleUser, Content: input},
	}, LoopOptions{
		Model:       a.model,
		Temperature: 0.4,
		MaxTokens:   512,
		MaxSteps:    4,
	})
	if err != nil {
		return "", 0, fmt.Errorf("%s loop: %w", a.role.Name, err)
	}
	return strings.TrimSpace(result.FinalMessage.Content), result.Usage.TotalTokens, nil
}

// ParseRoleJSON 解析角色输出的 JSON, 容错处理 (复用 ReAct 的提取逻辑).
func ParseRoleJSON(raw string) (map[string]any, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, fmt.Errorf("empty output")
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
		var m map[string]any
		if err := json.Unmarshal([]byte(c), &m); err == nil {
			return m, nil
		}
	}
	return nil, fmt.Errorf("cannot parse role JSON: %s", truncateForError(text, 100))
}

// IsApproved 从角色 JSON 输出中提取 approve 字段.
func IsApproved(parsed map[string]any) bool {
	if v, ok := parsed["approve"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetIssues 从角色 JSON 输出中提取 issues / violations 列表.
func GetIssues(parsed map[string]any, key string) []string {
	if v, ok := parsed[key]; ok {
		if arr, ok := v.([]any); ok {
			out := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}
