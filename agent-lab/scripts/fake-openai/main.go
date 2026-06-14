// Command fake-openai 启动一个台湾电商文案助理模拟器,
// 它不是真正的大模型, 而是根据 system prompt + 上下文生成合理的伪回复,
// 用于在没有本地大模型时直观跑通 agent-lab 的多轮对话效果.
//
// 行为:
//   1. 读取 system prompt 提取风格关键词 (亲切/促销/专业/年轻).
//   2. 从最近 2 轮 user 消息中提取商品相关信息 (品牌/价格/规格).
//   3. 生成带 emoji 的繁体中文文案或追问信息.
//   4. 当上下文较长 (>500 chars) 时, 在回复前返回一条 summary 事件.
//   5. 流式以字符为单位推送, 方便观察 SSE 效果.
//
// 用法:
//   go run ./agent-lab/scripts/fake-openai
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type chatReqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model    string           `json:"model"`
	Messages []chatReqMessage `json:"messages"`
	Stream   bool             `json:"stream"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", handleChat)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h1>fake-openai · 台湾电商文案助理</h1><p>POST /v1/chat/completions 即可使用.</p>")
	})

	addr := "127.0.0.1:18080"
	log.Printf("fake-openai (tw-ecom simulator) listening on http://%s/v1", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	reply := generateReply(req.Messages)

	if req.Stream {
		streamReply(w, req.Model, reply)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"model": req.Model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": reply},
			"finish_reason": "stop",
		}},
		"usage": map[string]int{
			"prompt_tokens":     approxTokens(req.Messages),
			"completion_tokens": len([]rune(reply)) / 2,
			"total_tokens":      approxTokens(req.Messages) + len([]rune(reply))/2,
		},
	})
}

// generateReply 根据最近消息生成符合角色卡的回复.
func generateReply(messages []chatReqMessage) string {
	var system string
	var users []string
	for _, m := range messages {
		switch m.Role {
		case "system":
			system = m.Content
		case "user":
			users = append(users, m.Content)
		}
	}
	style := detectStyle(system)
	last := ""
	if len(users) > 0 {
		last = strings.TrimSpace(users[len(users)-1])
	}
	info := extractProductInfo(users)

	// 如果用户在问候或没有提供商品信息, 用引导式回复.
	if isGreeting(last) {
		return "你好！我是台湾电商文案助理 🛍️ 可以告诉我你想写的商品名称、品牌、价位或目标平台吗？"
	}

	// 信息不足, 引导用户提供.
	if info.ProductName == "" {
		qa := []string{
			"好的～可以先告诉我以下信息吗？\n\n📦 商品名：\n🏷️ 品牌：\n💰 价位：\n🎯 目标平台（Shopee/PChome/小红书/自有站）：",
		}
		return qa[0]
	}

	// 信息充足, 生成文案.
	return makeCopy(style, info)
}

type productInfo struct {
	ProductName string
	Brand       string
	Price       string
	Platform    string
	Spec        string
	Keywords    []string
}

// extractProductInfo 从最近若干轮 user 消息里用正则粗略提取商品信息.
func extractProductInfo(users []string) productInfo {
	info := productInfo{}
	if len(users) == 0 {
		return info
	}
	text := strings.Join(users, "\n")

	// 商品名: 取 "商品是"、"产品"、"要写"、"卖的是" 后面的内容, 直到下一行或标点.
	namePatterns := []*regexp.Regexp{
		regexp.MustCompile(`商品(?:是|名|名称)?[：:\s]*([^\n，,。.!！?？]{2,30})`),
		regexp.MustCompile(`产(?:品|物)(?:名|名是)?[：:\s]*([^\n，,。.!！?？]{2,30})`),
		regexp.MustCompile(`想?卖(?:的是)?[：:\s]*([^\n，,。.!！?？]{2,30})`),
		regexp.MustCompile(`写(?:文案|一下)?[：:\s]*([^\n，,。.!！?？]{2,30})`),
	}
	for _, p := range namePatterns {
		if m := p.FindStringSubmatch(text); m != nil {
			info.ProductName = strings.TrimSpace(m[1])
			break
		}
	}
	if info.ProductName == "" {
		// 退化: 如果只有一行短内容, 整行当作商品名.
		if len(users) == 1 && len([]rune(users[0])) < 20 {
			info.ProductName = users[0]
		}
	}

	// 品牌: "品牌"/"牌子"/"XX出品".
	if m := regexp.MustCompile(`(?:品牌|牌子|出品|公司)[是：:\s]*([^\n，,。.!！?？\s]{2,20})`).FindStringSubmatch(text); m != nil {
		info.Brand = m[1]
	}

	// 价位.
	if m := regexp.MustCompile(`(?:价位|价格|售价|只要|NT\$|NT|台币|TWD|USD|RMB)[：:\s]*([^\n，,。.!！?？\s]{2,15})`).FindStringSubmatch(text); m != nil {
		info.Price = m[1]
	} else if m := regexp.MustCompile(`(\d{2,5}(?:\.\d{1,2})?\s*(?:元|NTD|NT|美金))`).FindStringSubmatch(text); m != nil {
		info.Price = m[1]
	}

	// 平台.
	platforms := []string{"Shopee", "PChome", "PCHOME", "小红书", "自有站", "蝦皮", "虾皮", "博客来", "Momo", "MOMO", "momo", "Yahoo", "YAHOO", "FB", "Facebook", "IG", "Instagram"}
	for _, p := range platforms {
		if strings.Contains(text, p) {
			info.Platform = p
			break
		}
	}

	// 规格.
	if m := regexp.MustCompile(`(?:规格|尺寸|大小|规格是)[：:\s]*([^\n，,。.!！?？]{2,30})`).FindStringSubmatch(text); m != nil {
		info.Spec = m[1]
	} else if m := regexp.MustCompile(`(\d+\s*x\s*\d+(?:\s*cm)?|\d+\s*[cm公分KGkg]+)`).FindStringSubmatch(text); m != nil {
		info.Spec = m[1]
	}

	// 关键词 (风格).
	kws := []string{"促销", "限量", "快闪", "周年庆", "新品", "亲切", "温暖", "专业", "年轻", "时髦", "送礼", "长辈", "情人", "母亲节"}
	for _, k := range kws {
		if strings.Contains(text, k) {
			info.Keywords = append(info.Keywords, k)
		}
	}

	return info
}

// detectStyle 从 system prompt 中提取风格关键词.
func detectStyle(system string) string {
	style := "亲切"
	if system == "" {
		return style
	}
	styles := map[string]string{
		"促销": "促销", "熱賣": "促销", "热卖": "促销",
		"专业": "专业", "嚴謹": "专业", "严谨": "专业",
		"年轻": "年轻", "年輕": "年轻", "潮流": "年轻", "时髦": "年轻",
		"溫暖": "温暖", "温暖": "温暖", "贴心": "温暖",
		"送礼": "送礼", "送禮": "送礼",
	}
	for key, val := range styles {
		if strings.Contains(system, key) {
			style = val
			break
		}
	}
	return style
}

func isGreeting(s string) bool {
	if s == "" {
		return false
	}
	greetings := []string{"你好", "您好", "hi", "HI", "Hi", "hello", "Hello", "HELLO", "嗨", "哈囉", "哈喽", "在吗", "在嗎", "早安", "午安", "晚安"}
	for _, g := range greetings {
		if strings.Contains(s, g) {
			return true
		}
	}
	return false
}

// makeCopy 根据风格和商品信息生成一段繁体中文文案.
func makeCopy(style string, info productInfo) string {
	name := info.ProductName
	brand := info.Brand
	if brand == "" {
		brand = "本舖"
	}
	price := info.Price
	platform := info.Platform
	if platform == "" {
		platform = "Shopee/PChome/小红书"
	}

	var head, body, cta string
	switch style {
	case "促销":
		head = "🔥 限时快闪！" + name + " 错过等一年！"
		body = fmt.Sprintf("【%s】严选原料，手感扎实，规格%s。数量有限，售完不补。", brand, fallback(info.Spec, "齐全"))
		cta = "👉 Shopee 现在下单：" + fallback(price, "请洽页面") + "，老客户再享 9 折优惠 ✨"
	case "专业":
		head = "📐 " + name + " · 专业规格说明"
		body = fmt.Sprintf("品牌：%s\n规格：%s\n适用对象：一般家庭 / 小资 / 送礼\n适用平台：%s\n我们推荐先看商品详情页与评价。", brand, fallback(info.Spec, "请补充"), platform)
		cta = "如需样品或批发，请联系客服 📮"
	case "年轻":
		head = "✨ 今天不买，明天涨价：" + name + "!"
		body = fmt.Sprintf("%s 出品，潮流单品一枚，规格%s。IG 网美已开箱，小红书话题飙升 📈", brand, fallback(info.Spec, "多样"))
		cta = "👉 Shopee/PChome 搜【" + name + "】，第一波出货中 🚀"
	case "送礼":
		head = "🎁 送礼首选 · " + name
		body = fmt.Sprintf("%s 出品，包装精美，适合年节送礼、客户感谢、家庭日常使用。规格%s，多种颜色可选。", brand, fallback(info.Spec, "齐全"))
		cta = "👉 现在下单免运，满额再送精美好物 🎊"
	default: // 亲切
		head = "🛍️ 【" + name + "】日常必备的好选择！"
		body = fmt.Sprintf("来自 %s，品质稳定、回头客高。规格%s，适合居家/送礼。在 %s 都有上架。", brand, fallback(info.Spec, "齐全"), platform)
		cta = "👉 点进商品页看看真实评价，心动不如行动 💬"
	}

	// 信息不足时追加追问.
	ask := ""
	if info.Price == "" || info.Platform == "" || info.Spec == "" {
		ask = "\n\n💡 小提醒：可以再告诉我 "
		if info.Price == "" {
			ask += "价位/ "
		}
		if info.Platform == "" {
			ask += "目标平台/ "
		}
		if info.Spec == "" {
			ask += "规格/ "
		}
		ask = strings.TrimSuffix(ask, "/ ")
		ask += " ，我可以写得更准～"
	}

	return head + "\n\n" + body + "\n\n" + cta + ask
}

func fallback(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func approxTokens(messages []chatReqMessage) int {
	total := 0
	for _, m := range messages {
		total += len([]rune(m.Content)) / 2
	}
	if total < 10 {
		total = 10
	}
	return total
}

func streamReply(w http.ResponseWriter, model, reply string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(event string, payload string) {
		if event != "" {
			fmt.Fprintf(w, "event: %s\n", event)
		}
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}

	runes := []rune(reply)
	// 按字符分片流式.
	var acc strings.Builder
	for _, r := range runes {
		acc.WriteRune(r)
		// 每 2~4 个字符推一次, 再加点抖动, 看起来更像流式.
		if acc.Len() >= 3 {
			chunk, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{
					"index": 0, "delta": map[string]any{"role": "assistant", "content": acc.String()},
					"finish_reason": "",
				}},
			})
			send("", string(chunk))
			acc.Reset()
			time.Sleep(18 * time.Millisecond)
		}
	}
	if acc.Len() > 0 {
		chunk, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{
				"index": 0, "delta": map[string]any{"role": "assistant", "content": acc.String()},
				"finish_reason": "",
			}},
		})
		send("", string(chunk))
	}

	final, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{
			"index": 0, "delta": map[string]any{}, "finish_reason": "stop",
		}},
		"usage": map[string]int{"prompt_tokens": approxTokens([]chatReqMessage{{Role: "system", Content: "sys"}}), "completion_tokens": len(runes) / 2, "total_tokens": approxTokens([]chatReqMessage{{Role: "system", Content: "sys"}}) + len(runes)/2},
	})
	send("", string(final))
	send("", "[DONE]")
}
