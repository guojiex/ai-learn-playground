// Package prompt 提供电商文案助理的角色卡 (persona) 与模板化 prompt.
package prompt

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed personas/*.md
var personaFS embed.FS

// Persona 描述一个角色卡.
type Persona struct {
	Name        string
	SystemPrompt string
}

// Default 是默认的台湾电商文案助理角色卡.
func Default() Persona {
	return Persona{
		Name: "tw-ecom-copywriter",
		SystemPrompt: `你是一个稳重务实的台湾电商文案助理.

你的任务: 帮店主把 SKU 信息转成适合 Shopee / PChome / 小红书风格的商品文案.

工作方式:
1. 先收集 SKU 必要信息 (商品名/规格/材质/卖点/品牌/价位/目标平台), 信息不全就追问.
2. 如果用户给了风格关键词 (例如 "亲切" / "促销" / "专业" / "年轻"), 按风格调整语气.
3. 输出时遵守:
   - 第一段 20 字内点出核心卖点, 加表情符号.
   - 第二段描述商品与规格, 不超过 80 字.
   - 第三段给一句 CTA (行动呼吁).

注意:
- 使用繁体中文.
- 不要写 "您好很高兴为您服务" 这类虚话.
- 不输出价格的货币单位以外的数字解释.`,
	}
}

// LoadPersona 从 personas/<name>.md 读取角色卡. 文件第一行作为 name, 其余作为 system prompt.
// 如果文件不存在, 返回 Default.
func LoadPersona(name string) (Persona, error) {
	if name == "" {
		return Default(), nil
	}
	data, err := personaFS.ReadFile("personas/" + name + ".md")
	if err != nil {
		return Default(), fmt.Errorf("load persona %q: %w", name, err)
	}
	text := strings.TrimSpace(string(data))
	lines := strings.SplitN(text, "\n", 2)
	system := ""
	if len(lines) > 1 {
		system = strings.TrimSpace(lines[1])
	} else {
		system = text
	}
	return Persona{
		Name:         name,
		SystemPrompt: system,
	}, nil
}
