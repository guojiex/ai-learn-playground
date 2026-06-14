package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"ai-learn-playground/agent-lab/internal/llm"
)

// PlatformLint 检查电商文案是否符合各平台限制 (字数 / 敏感词 / 标签数).
//
// 平台规则 (M2 演示用, 真实规则会随平台变更):
//   shopee_tw: 标题 ≤ 60 字; 禁词: 最高/第一/绝对; 标签 (#) ≤ 8.
//   pchome:    标题 ≤ 50 字; 禁词: 最便宜/限定独家; 标签 ≤ 5.
//   momo:      标题 ≤ 55 字; 禁词: 第一/独家; 标签 ≤ 6.
type PlatformLint struct{}

// NewPlatformLint 构造工具.
func NewPlatformLint() *PlatformLint { return &PlatformLint{} }

// Schema 实现 Tool.
func (PlatformLint) Schema() llm.ToolSchema {
	return Schema(
		"platform_lint",
		"按目标平台校验电商文案 (标题/正文): 字数上限、敏感词、标签数上限. 返回 ok / 违规清单. 用于在生成最终文案前做安全检查.",
		map[string]any{
			"type": "object",
			"required": []string{"platform", "text"},
			"properties": map[string]any{
				"platform": map[string]any{"type": "string", "enum": []string{"shopee_tw", "pchome", "momo"}},
				"text":     map[string]any{"type": "string", "description": "要校验的文本 (标题或正文)"},
				"kind":     map[string]any{"type": "string", "enum": []string{"title", "body"}, "description": "默认 title"},
			},
		},
	)
}

type platformLintArgs struct {
	Platform string `json:"platform"`
	Text     string `json:"text"`
	Kind     string `json:"kind"`
}

type platformRule struct {
	maxTitle int
	maxBody  int
	maxTags  int
	banned   []string
}

var platformRules = map[string]platformRule{
	"shopee_tw": {maxTitle: 60, maxBody: 600, maxTags: 8, banned: []string{"最高", "第一", "绝对", "絕對"}},
	"pchome":    {maxTitle: 50, maxBody: 500, maxTags: 5, banned: []string{"最便宜", "限定独家", "限定獨家"}},
	"momo":      {maxTitle: 55, maxBody: 550, maxTags: 6, banned: []string{"第一", "独家", "獨家"}},
}

// Invoke 实现 Tool.
func (PlatformLint) Invoke(_ context.Context, raw json.RawMessage) (string, error) {
	var args platformLintArgs
	if err := ParseArgs(raw, &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Platform) == "" {
		return "", fmt.Errorf("platform is required")
	}
	if strings.TrimSpace(args.Text) == "" {
		return "", fmt.Errorf("text is required")
	}
	rule, ok := platformRules[args.Platform]
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s", args.Platform)
	}
	if args.Kind == "" {
		args.Kind = "title"
	}

	violations := []map[string]any{}
	limit := rule.maxTitle
	if args.Kind == "body" {
		limit = rule.maxBody
	}
	length := utf8.RuneCountInString(args.Text)
	if length > limit {
		violations = append(violations, map[string]any{
			"code":    "length_exceeded",
			"message": fmt.Sprintf("%s 上限 %d 字, 当前 %d", args.Kind, limit, length),
		})
	}
	for _, w := range rule.banned {
		if strings.Contains(args.Text, w) {
			violations = append(violations, map[string]any{
				"code":    "banned_word",
				"word":    w,
				"message": fmt.Sprintf("命中禁词 %q", w),
			})
		}
	}
	tagCount := strings.Count(args.Text, "#")
	if tagCount > rule.maxTags {
		violations = append(violations, map[string]any{
			"code":    "too_many_tags",
			"message": fmt.Sprintf("标签数 %d 超过上限 %d", tagCount, rule.maxTags),
		})
	}
	report := map[string]any{
		"ok":         len(violations) == 0,
		"platform":   args.Platform,
		"kind":       args.Kind,
		"length":     length,
		"limit":      limit,
		"tag_count":  tagCount,
		"violations": violations,
	}
	out, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
