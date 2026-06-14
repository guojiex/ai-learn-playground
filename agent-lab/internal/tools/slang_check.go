package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ai-learn-playground/agent-lab/internal/llm"
)

// SlangCheck 衡量文案的"台湾电商黑话密度", 帮助模型判断是否要在 cta 加常用词.
//
// 这里"黑话"是台湾电商常见的口语 / 促销词, 例如 现貨/免運/限時/必買/CP值 等.
type SlangCheck struct{}

// NewSlangCheck 构造工具.
func NewSlangCheck() *SlangCheck { return &SlangCheck{} }

// 常见黑话词典 (繁体优先, 简体作为别名).
var slangWords = []string{
	"現貨", "现货", "免運", "免运", "限時", "限时",
	"必買", "必买", "CP值", "性價比", "性价比",
	"熱賣", "热卖", "下殺", "下杀", "搶購", "抢购",
	"超值", "破盤", "破盘", "首發", "首发",
	"獨家", "独家", "新品", "好評", "好评",
}

// Schema 实现 Tool.
func (SlangCheck) Schema() llm.ToolSchema {
	return Schema(
		"slang_check",
		"统计文案中台湾电商黑话 (現貨/免運/限時/必買/CP值 等) 的命中数与密度, 返回命中清单和每千字密度. 用于辅助判断是否需要追加促销口号.",
		map[string]any{
			"type":     "object",
			"required": []string{"text"},
			"properties": map[string]any{
				"text": map[string]any{"type": "string", "description": "要分析的文案"},
			},
		},
	)
}

type slangCheckArgs struct {
	Text string `json:"text"`
}

// Invoke 实现 Tool.
func (SlangCheck) Invoke(_ context.Context, raw json.RawMessage) (string, error) {
	var args slangCheckArgs
	if err := ParseArgs(raw, &args); err != nil {
		return "", err
	}
	text := strings.TrimSpace(args.Text)
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	hits := map[string]int{}
	total := 0
	for _, w := range slangWords {
		c := strings.Count(text, w)
		if c > 0 {
			hits[w] = c
			total += c
		}
	}
	chars := len([]rune(text))
	var density float64
	if chars > 0 {
		density = float64(total) / float64(chars) * 1000
	}
	out := map[string]any{
		"chars":         chars,
		"total_hits":    total,
		"density_per_k": roundFloat(density, 2),
		"hits":          hits,
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func roundFloat(f float64, digits int) float64 {
	shift := 1.0
	for i := 0; i < digits; i++ {
		shift *= 10
	}
	return float64(int64(f*shift+0.5)) / shift
}
