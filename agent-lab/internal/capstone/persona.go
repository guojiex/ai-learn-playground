// Package capstone 把 M0-M10 的能力组合成完整电商文案 Agent (M11).
//
// 流水线: 输入(seller/sku/platforms/style) → Multi-Agent 协作(M7) → 评测(M9) → 多平台输出
package capstone

// DefaultPersona 是 Capstone 默认的电商文案助理人设.
const DefaultPersona = `你是台湾电商文案专家团队的总协调员. 团队由 4 个角色组成:
- Researcher: 调研商品卖点与竞品
- Writer: 撰写平台文案 (标题+正文+标签)
- Critic: 评审文案吸引力与完整性
- Compliance: 检查平台合规 (违禁词/字数)

你的目标是: 根据卖家需求, 协调团队产出多平台高质量文案.
输出必须是繁体中文, 符合台湾电商风格 (現貨/免運/CP值 等黑话).`

// StylePersona 返回不同风格的 system prompt 补充.
func StylePersona(style string) string {
	switch style {
	case "girlfriend":
		return "语气亲切如闺蜜推荐, 多用 emoji, 种草感强."
	case "promo":
		return "促销风格, 强调限时/下杀/免运, 制造紧迫感."
	case "pro":
		return "专业风格, 突出规格/材质/认证, 信任感强."
	case "gift":
		return "送礼风格, 强调包装/质感/心意, 适合节日送礼场景."
	default:
		return "自然亲切的电商文案风格."
	}
}

// PlatformName 返回平台的中文显示名.
func PlatformName(platform string) string {
	switch platform {
	case "shopee", "shopee_tw":
		return "蝦皮台湾"
	case "momo":
		return "momo購物網"
	case "pchome":
		return "PChome 24h"
	case "xhs", "xiaohongshu", "xhs_tw":
		return "小红书台湾"
	default:
		return platform
	}
}
