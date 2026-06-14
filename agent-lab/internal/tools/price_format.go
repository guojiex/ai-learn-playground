package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ai-learn-playground/agent-lab/internal/llm"
)

// PriceFormat 把价格 + 配送/促销 标签拼成一段标准化的台币展示串.
//
// 例如: NT$690 · 現貨 · 限時免運.
type PriceFormat struct{}

// NewPriceFormat 构造 PriceFormat 工具.
func NewPriceFormat() *PriceFormat { return &PriceFormat{} }

// Schema 实现 Tool.
func (PriceFormat) Schema() llm.ToolSchema {
	return Schema(
		"price_format",
		"把数字价格与配送/促销标签拼成台币标准展示串, 例如 'NT$690 · 現貨 · 限時免運'. 用于在写商品文案的促销行时格式化价格.",
		map[string]any{
			"type": "object",
			"required": []string{"price_twd"},
			"properties": map[string]any{
				"price_twd": map[string]any{"type": "number", "description": "台币价格 (整数或小数)"},
				"shipping":  map[string]any{"type": "string", "description": "配送描述, 例如 '現貨 / 24h 出貨'"},
				"badges":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "促销/卖点标签, 例如 ['限時免運','滿千折百']"},
			},
		},
	)
}

type priceFormatArgs struct {
	PriceTWD float64  `json:"price_twd"`
	Shipping string   `json:"shipping"`
	Badges   []string `json:"badges"`
}

// Invoke 实现 Tool.
func (PriceFormat) Invoke(_ context.Context, raw json.RawMessage) (string, error) {
	var args priceFormatArgs
	if err := ParseArgs(raw, &args); err != nil {
		return "", err
	}
	if args.PriceTWD <= 0 {
		return "", fmt.Errorf("price_twd must be > 0")
	}
	parts := []string{formatTWD(args.PriceTWD)}
	if s := strings.TrimSpace(args.Shipping); s != "" {
		parts = append(parts, s)
	}
	for _, b := range args.Badges {
		b = strings.TrimSpace(b)
		if b != "" {
			parts = append(parts, b)
		}
	}
	out := map[string]any{
		"display": strings.Join(parts, " · "),
		"price":   args.PriceTWD,
		"parts":   parts,
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatTWD(p float64) string {
	if p == float64(int64(p)) {
		return fmt.Sprintf("NT$%d", int64(p))
	}
	return fmt.Sprintf("NT$%.2f", p)
}
