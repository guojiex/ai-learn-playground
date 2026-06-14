package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ai-learn-playground/agent-lab/internal/llm"
)

// Product 是商品记录的数据形态.
type Product struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Brand      string   `json:"brand"`
	Origin     string   `json:"origin,omitempty"`
	Spec       string   `json:"spec,omitempty"`
	PriceTWD   int      `json:"price_twd"`
	Shipping   string   `json:"shipping,omitempty"`
	Highlights []string `json:"highlights,omitempty"`
	Platforms  []string `json:"platforms,omitempty"`
}

// ProductLookup 通过 ID 或关键词查询产品库.
//
// 数据来源: dataDir/products.json. 支持两种查询:
//   - 按 id 精确查找;
//   - 按 query 模糊匹配 name/brand/highlights, 返回最多 3 条.
type ProductLookup struct {
	mu       sync.RWMutex
	dataDir  string
	cache    []Product
	cacheKey string // 文件指纹, 简单用 size+modtime 字符串.
}

// NewProductLookup 用给定的 dataDir 构造工具. dataDir 应包含 products.json.
func NewProductLookup(dataDir string) *ProductLookup {
	return &ProductLookup{dataDir: dataDir}
}

// Schema 实现 Tool.
func (p *ProductLookup) Schema() llm.ToolSchema {
	return Schema(
		"product_lookup",
		"按商品 ID 或关键词查询商品库. 至少要提供 id 或 query 之一. 返回 JSON 数组, 每个元素包含 id/name/brand/origin/spec/price_twd/shipping/highlights/platforms. 用于在写文案前补充准确的商品事实.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":    map[string]any{"type": "string", "description": "商品 ID, 例如 sku_001"},
				"query": map[string]any{"type": "string", "description": "关键词模糊匹配 name/brand/highlights"},
				"limit": map[string]any{"type": "integer", "description": "最多返回条数, 默认 3", "minimum": 1, "maximum": 10},
			},
		},
	)
}

type productLookupArgs struct {
	ID    string `json:"id"`
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// Invoke 实现 Tool.
func (p *ProductLookup) Invoke(ctx context.Context, raw json.RawMessage) (string, error) {
	var args productLookupArgs
	if err := ParseArgs(raw, &args); err != nil {
		return "", err
	}
	args.ID = strings.TrimSpace(args.ID)
	args.Query = strings.TrimSpace(args.Query)
	if args.ID == "" && args.Query == "" {
		return "", fmt.Errorf("either id or query is required")
	}
	if args.Limit <= 0 {
		args.Limit = 3
	}
	if args.Limit > 10 {
		args.Limit = 10
	}

	all, err := p.load()
	if err != nil {
		return "", err
	}

	hits := matchProducts(all, args.ID, args.Query, args.Limit)
	out, err := json.Marshal(hits)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func matchProducts(all []Product, id, query string, limit int) []Product {
	if id != "" {
		for _, p := range all {
			if p.ID == id {
				return []Product{p}
			}
		}
		return []Product{}
	}
	q := strings.ToLower(query)
	out := make([]Product, 0, limit)
	for _, p := range all {
		if matchString(p.Name, q) || matchString(p.Brand, q) || matchHighlights(p.Highlights, q) {
			out = append(out, p)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func matchString(s, q string) bool {
	if q == "" {
		return false
	}
	return strings.Contains(strings.ToLower(s), q)
}

func matchHighlights(hs []string, q string) bool {
	for _, h := range hs {
		if matchString(h, q) {
			return true
		}
	}
	return false
}

func (p *ProductLookup) load() ([]Product, error) {
	path := filepath.Join(p.dataDir, "products.json")
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("products.json not found at %s: %w", path, err)
	}
	key := fmt.Sprintf("%d-%d", st.Size(), st.ModTime().UnixNano())

	p.mu.RLock()
	if p.cacheKey == key && p.cache != nil {
		out := p.cache
		p.mu.RUnlock()
		return out, nil
	}
	p.mu.RUnlock()

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var arr []Product
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("decode products.json: %w", err)
	}

	p.mu.Lock()
	p.cache = arr
	p.cacheKey = key
	p.mu.Unlock()
	return arr, nil
}
