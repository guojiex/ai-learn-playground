// Command agent 是 agent Lab 的 CLI 入口.
//
// 支持两种模式:
//
//   - native: 依赖 OpenAI 兼容 server 的原生 function-calling (M2).
//   - react:  自写 JSON 协议的 Thought-Action-Observation 循环 (M3).
//
// 用法:
//
//	export OPENAI_BASE_URL=http://127.0.0.1:8080/v1
//	export OPENAI_API_KEY=sk-local
//	export AGENTLAB_PROFILE=L
//	go run ./agent-lab/cmd/agent -mode react -m "帮我为 sku_001 写一段蝦皮标题"
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"ai-learn-playground/agent-lab/internal/agent"
	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/prompt"
	"ai-learn-playground/agent-lab/internal/tools"
)

func main() {
	var (
		modelOver string
		message   string
		dataDir   string
		mode      string
		maxSteps  int
		maxTokens int
		temp      float64
	)
	flag.StringVar(&modelOver, "model", "", "覆盖 AGENTLAB_MODEL_CHAT")
	flag.StringVar(&message, "m", "", "用户消息 (必填)")
	flag.StringVar(&dataDir, "data", "agent-lab/data/products", "products.json 所在目录")
	flag.StringVar(&mode, "mode", "native", "agent 模式: native | react")
	flag.IntVar(&maxSteps, "max-steps", 8, "最大 agent 步数")
	flag.IntVar(&maxTokens, "max-tokens", 512, "每次 LLM 调用的输出上限")
	flag.Float64Var(&temp, "temperature", 0.4, "采样温度")
	flag.Parse()

	if strings.TrimSpace(message) == "" {
		fmt.Fprintln(os.Stderr, "usage: agent -m <message> [-mode=native|react]")
		os.Exit(2)
	}
	switch mode {
	case "native", "react":
	default:
		fmt.Fprintln(os.Stderr, "unknown -mode:", mode)
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
	baseSystem := persona.SystemPrompt + "\n\n你可以调用以下工具来获取事实或校验文案: product_lookup / price_format / platform_lint / slang_check. 在不确定商品事实时优先调用 product_lookup."

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	opts := agent.Options{
		SystemPrompt: baseSystem,
		Model:        model,
		Temperature:  float32(temp),
		MaxTokens:    maxTokens,
		MaxSteps:     maxSteps,
	}

	var a agent.Agent
	switch mode {
	case "react":
		a = agent.NewReActAgent(client, reg, opts)
	default:
		a = agent.NewNativeAgent(client, reg, opts)
	}

	fmt.Fprintf(os.Stderr, "[agent] profile=%s mode=%s model=%s tools=%v max_steps=%d\n",
		cfg.Profile, a.Mode(), model, reg.Names(), maxSteps)

	res, err := a.Run(ctx, message)
	for _, s := range res.Steps {
		prefix := fmt.Sprintf("[step#%d %s]", s.StepIndex, s.Kind)
		switch s.Kind {
		case agent.StepAction:
			args := s.ActionArgs
			if len([]rune(args)) > 80 {
				args = string([]rune(args)[:80]) + "..."
			}
			obs := truncate(s.Observation, 160)
			fmt.Fprintf(os.Stderr, "%s thought=%q => %s(%s)\n", prefix, truncate(s.Thought, 60), s.ActionName, args)
			if s.Error != "" {
				fmt.Fprintf(os.Stderr, "%s   obs=ERR: %s\n", prefix, truncate(s.Error, 120))
			} else {
				fmt.Fprintf(os.Stderr, "%s   obs=%s\n", prefix, obs)
			}
		case agent.StepFinal:
			fmt.Fprintf(os.Stderr, "%s final emitted\n", prefix)
		case agent.StepParseRetry:
			fmt.Fprintf(os.Stderr, "%s parse fail (retry): %s\n", prefix, truncate(s.Error, 120))
		case agent.StepParseDegrade:
			fmt.Fprintf(os.Stderr, "%s parse fail (degrade to raw text): %s\n", prefix, truncate(s.Thought, 120))
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent error:", err)
	}

	fmt.Println(strings.TrimSpace(res.Final))
	fmt.Fprintf(os.Stderr, "[usage] mode=%s steps=%d prompt=%d completion=%d total=%d elapsed=%s\n",
		res.Mode, len(res.Steps), res.Usage.PromptTokens, res.Usage.CompletionTokens, res.Usage.TotalTokens, res.Elapsed)
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}
