package llm

import "fmt"

// RouteMatch 描述路由规则的匹配条件 (M10).
type RouteMatch struct {
	Task        string `json:"task,omitempty"`          // 任务标签, 例如 "title" / "plan" / "body"
	CtxTokensGT int    `json:"ctx_tokens_gt,omitempty"` // 上下文 token 数超过此值时匹配
}

// RouteRule 是一条路由规则 (M10).
type RouteRule struct {
	Match    RouteMatch `json:"match"`
	Use      string     `json:"use"`      // 使用的 tag, 例如 "fast" / "reason" / "default"
	Fallback []string   `json:"fallback"` // 失败降级的 tag 链, 例如 ["default"]
}

// Policy 是路由策略的集合 (M10).
type Policy struct {
	Rules []RouteRule `json:"routes"`
}

// DefaultPolicy 返回默认路由策略.
func DefaultPolicy() *Policy {
	return &Policy{
		Rules: []RouteRule{
			{Match: RouteMatch{Task: "title"}, Use: "fast", Fallback: []string{"default"}},
			{Match: RouteMatch{Task: "tag"}, Use: "fast", Fallback: []string{"default"}},
			{Match: RouteMatch{Task: "plan"}, Use: "reason", Fallback: []string{"default"}},
			{Match: RouteMatch{Task: "planner"}, Use: "reason", Fallback: []string{"default"}},
			{Match: RouteMatch{Task: "body"}, Use: "default"},
			{Match: RouteMatch{Task: "critic"}, Use: "default"},
			{Match: RouteMatch{Task: "compliance"}, Use: "fast", Fallback: []string{"default"}},
			{Match: RouteMatch{CtxTokensGT: 6000}, Use: "default"},
		},
	}
}

// RouteResult 是一次路由决策的结果.
type RouteResult struct {
	Primary   *ModelEntry   // 首选模型
	Fallbacks []*ModelEntry // 降级链
	Reason    string        // 为什么选这个路由
	RuleIdx   int           // 命中的规则序号 (-1 = 默认)
}

// Evaluate 根据任务标签和上下文长度, 返回路由结果.
func (p *Policy) Evaluate(task string, ctxTokens int, reg *Registry) (*RouteResult, error) {
	for i, rule := range p.Rules {
		if rule.Match.Task != "" && rule.Match.Task == task {
			return p.resolve(rule, reg, fmt.Sprintf("match=task:%s", task), i)
		}
		if rule.Match.CtxTokensGT > 0 && ctxTokens > rule.Match.CtxTokensGT {
			return p.resolve(rule, reg, fmt.Sprintf("match=ctx_tokens_gt:%d (actual=%d)", rule.Match.CtxTokensGT, ctxTokens), i)
		}
	}
	// 无匹配规则 → 用 default tag.
	primary, ok := reg.ByTag("default")
	if !ok && len(reg.Models) > 0 {
		primary = &reg.Models[0]
	}
	if primary == nil {
		return nil, fmt.Errorf("no model available for task=%s", task)
	}
	return &RouteResult{
		Primary: primary,
		Reason:  "default (no rule matched)",
		RuleIdx: -1,
	}, nil
}

func (p *Policy) resolve(rule RouteRule, reg *Registry, reason string, idx int) (*RouteResult, error) {
	primary, ok := reg.ByTag(rule.Use)
	if !ok {
		return nil, fmt.Errorf("no model with tag=%s", rule.Use)
	}
	result := &RouteResult{
		Primary: primary,
		Reason:  reason + " → tag:" + rule.Use + " → " + primary.Name,
		RuleIdx: idx,
	}
	for _, fb := range rule.Fallback {
		if m, ok := reg.ByTag(fb); ok {
			result.Fallbacks = append(result.Fallbacks, m)
		}
	}
	return result, nil
}
