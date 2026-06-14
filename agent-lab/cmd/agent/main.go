// Command agent 是 M2 的 CLI 入口: 带工具的多轮 agent.
//
// 它基于本地 OpenAI 兼容 server 的原生 function-calling 能力, 让模型主动调用
// product_lookup / price_format / platform_lint / slang_check 四个工具.
//
// 用法:
//
//	export OPENAI_BASE_URL=http://127.0.0.1:8080/v1
//	export OPENAI_API_KEY=sk-local
//	export AGENTLAB_PROFILE=L
//	go run ./agent-lab/cmd/agent -m "帮我为 sku_001 写一段蝦皮标题, 要带現貨/免運"
//
// 注意: fake-openai server 不支持 tool_calls, 该命令需要真正具备 function calling 能力的模型 (Qwen2.5-Instruct 等).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ai-learn-playground/agent-lab/internal/agent"
	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/prompt"
	"ai-learn-playground/agent-lab/internal/tools"
)

func main() {
	var (
		modelOver  string
		message    string
		dataDir    string
		maxSteps   int
		maxTokens  int
		temp       float64
	)
	flag.StringVar(&modelOver, "model", "", "覆盖 AGENTLAB_MODEL_CHAT")
	flag.StringVar(&message, "m", "", "用户消息 (必填)")
	flag.StringVar(&dataDir, "data", "agent-lab/data/products", "products.json 所在目录")
	flag.IntVar(&maxSteps, "max-steps", 8, "最大 agent 步数")
	flag.IntVar(&maxTokens, "max-tokens", 512, "每次 LLM 调用的输出上限")
	flag.Float64Var(&temp, "temperature", 0.4, "采样温度")
	flag.Parse()

	if strings.TrimSpace(message) == "" {
		fmt.Fprintln(os.Stderr, "usage: agent -m <message>")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	model := cfg.ModelChat
	if modelOver != "" {
		model = modelOver
	}

	client := llm.NewOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout, llm.WithMaxRetries(cfg.MaxRetries))

	reg := tools.NewRegistry()
	reg.Register(tools.NewProductLookup(dataDir))
	reg.Register(tools.NewPriceFormat())
	reg.Register(tools.NewPlatformLint())
	reg.Register(tools.NewSlangCheck())

	persona := prompt.Default()
	system := persona.SystemPrompt + "\n\n你可以调用以下工具来获取事实或校验文案: product_lookup / price_format / platform_lint / slang_check. 在不确定商品事实时优先调用 product_lookup."

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "[agent] profile=%s model=%s tools=%v max_steps=%d\n", cfg.Profile, model, reg.Names(), maxSteps)

	res, err := agent.Loop(ctx, client, reg, []llm.Message{
		{Role: llm.RoleSystem, Content: system},
		{Role: llm.RoleUser, Content: message},
	}, agent.LoopOptions{
		Model:       model,
		Temperature: float32(temp),
		MaxTokens:   maxTokens,
		MaxSteps:    maxSteps,
	})

	for _, r := range res.ToolCalls {
		args := compactJSON(r.Args)
		preview := r.Result
		if r.Err != "" {
			preview = "ERR: " + r.Err
		}
		preview = truncate(preview, 200)
		fmt.Fprintf(os.Stderr, "[tool#%d] %s(%s) -> %s  (%dms)\n",
			r.StepIndex, r.Name, args, preview, r.DurationMS)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent error:", err)
		os.Exit(1)
	}

	fmt.Println(strings.TrimSpace(res.FinalMessage.Content))
	fmt.Fprintf(os.Stderr, "[usage] steps=%d prompt=%d completion=%d total=%d elapsed=%s\n",
		res.Steps, res.Usage.PromptTokens, res.Usage.CompletionTokens, res.Usage.TotalTokens, time.Since(time.Now()))
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "..."
}
