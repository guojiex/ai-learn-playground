// Command route 演示模型路由 (M10).
//
// 用法:
//
//	go run ./agent-lab/cmd/route --task title -m "为毛巾写标题"
//	go run ./agent-lab/cmd/route --task plan  -m "为 sku_001 规划上新"
//	go run ./agent-lab/cmd/route --task body  -m "为毛巾写正文" --huge-context
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
)

func main() {
	var (
		task        string
		message     string
		hugeContext bool
		configPath  string
	)
	flag.StringVar(&task, "task", "title", "任务标签: title / tag / plan / body / critic / compliance")
	flag.StringVar(&message, "m", "", "消息内容")
	flag.BoolVar(&hugeContext, "huge-context", false, "模拟超长上下文 (>6000 tokens)")
	flag.StringVar(&configPath, "config", "agent-lab/config/models.json", "模型注册表 JSON")
	flag.Parse()

	if message == "" {
		message = "为 sku_001 写一篇蝦皮台湾文案"
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	// 加载注册表 (失败时用默认).
	var reg *llm.Registry
	if reg, err = llm.LoadRegistry(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "[router] 使用默认注册表: %v\n", err)
		reg = llm.DefaultRegistry(cfg.BaseURL, cfg.Profile)
	} else {
		// 覆盖 base_url 为环境变量值.
		for i := range reg.Models {
			if reg.Models[i].BaseURL == "" {
				reg.Models[i].BaseURL = cfg.BaseURL
			}
		}
	}

	policy := llm.DefaultPolicy()
	if reg.Profile != "" && len(reg.Models) > 0 {
		// 如果 JSON 里有 routes, 直接用; 否则用默认 policy.
	}

	client := llm.NewOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout, llm.WithMaxRetries(cfg.MaxRetries))
	router := llm.NewRouter(reg, policy, client)

	// 如果 huge-context, 注入一段长文本.
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: message},
	}
	if hugeContext {
		msgs = []llm.Message{
			{Role: llm.RoleSystem, Content: strings.Repeat("这是一段很长的上下文. ", 500)},
			{Role: llm.RoleUser, Content: message},
		}
	}

	// 先打印路由决策.
	result, err := router.Route(task, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[router] route error:", err)
		os.Exit(1)
	}
	fmt.Printf("[router] %s → %s\n", result.Reason, result.Primary.Name)

	// 执行调用.
	ctx := context.Background()
	resp, rec, err := router.ChatForTask(ctx, task, llm.ChatRequest{
		Messages: msgs,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[router] 调用失败: %v\n", err)
		fmt.Fprintf(os.Stderr, "[router] record: %+v\n", rec)
		os.Exit(1)
	}

	fmt.Printf("\n--- 模型回复 (%s, %dms) ---\n%s\n", resp.Model, rec.LatencyMs, resp.Message.Content)
	fmt.Printf("\n[router] chosen=%s fallbacks=%v success=%v tokens=%d\n",
		rec.Chosen, rec.Fallbacks, rec.Success, resp.Usage.TotalTokens)
}
