package eval

import (
	"strings"
)

// Metrics 计算 agent 输出的业务度量 (M9).
//
// 不依赖 LLM, 纯规则计算:
//   - SlangHit: 台湾电商黑话命中率 (0-1).
//   - ComplianceOK: 平台合规检查 (违禁词/字数).
type Metrics struct {
	slangTerms  []string
	bannedTerms []string
	maxTitleLen map[string]int
}

// NewMetrics 构造一个度量器, 内置常见黑话与违禁词.
func NewMetrics() *Metrics {
	return &Metrics{
		slangTerms: []string{
			"現貨", "免運", "出貨", "小資", "必買", "回購", "推", "狂推", "CP值",
			"下殺", "限時", "秒殺", "爆款", "神級", "佛心", "親子", "團購",
		},
		bannedTerms: []string{
			"最便宜", "最低價", "全網第一", "保證治癒", "根治", "特效藥",
			"致癌", "有毒", "破盤價", "跳樓價",
		},
		maxTitleLen: map[string]int{
			"shopee":      120,
			"pchome":      50,
			"momo":        60,
			"xiaohongshu": 20,
		},
	}
}

// SlangHit 返回文案中命中的黑话比例 (0-1).
// 命中越多说明越"接地气".
func (m *Metrics) SlangHit(text string) float64 {
	if text == "" {
		return 0
	}
	lower := strings.ToLower(text)
	hit := 0
	for _, term := range m.slangTerms {
		if strings.Contains(lower, strings.ToLower(term)) {
			hit++
		}
	}
	// 命中 5 个以上就满分.
	if hit >= 5 {
		return 1.0
	}
	return float64(hit) / 5.0
}

// ComplianceOK 检查文案是否符合平台合规规则.
// 检查: 无违禁词 + (如有标题行) 标题字数不超限.
func (m *Metrics) ComplianceOK(text, platform string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	for _, banned := range m.bannedTerms {
		if strings.Contains(lower, strings.ToLower(banned)) {
			return false
		}
	}
	return true
}

// SlangHits 返回命中的具体黑话列表 (供 debug).
func (m *Metrics) SlangHits(text string) []string {
	lower := strings.ToLower(text)
	var hits []string
	for _, term := range m.slangTerms {
		if strings.Contains(lower, strings.ToLower(term)) {
			hits = append(hits, term)
		}
	}
	return hits
}

// BannedHits 返回命中的违禁词列表.
func (m *Metrics) BannedHits(text string) []string {
	lower := strings.ToLower(text)
	var hits []string
	for _, banned := range m.bannedTerms {
		if strings.Contains(lower, strings.ToLower(banned)) {
			hits = append(hits, banned)
		}
	}
	return hits
}
