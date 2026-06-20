// Command multi 是 agent-lab 的 Multi-Agent CLI 入口 (M7).
//
// 4 个角色 (Researcher → Writer → Critic + Compliance) 多轮协作,
// 直到 Critic+Compliance 都 approve 或达到 maxRounds.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-learn-playground/agent-lab/internal/agent"
	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

func main() {
	var (
		goal      string
		maxRounds int
		dump      string
		dataDir   string
	)
	flag.StringVar(&goal, "m", "", "目标 (必填)")
	flag.IntVar(&maxRounds, "rounds", 4, "最大轮次")
	flag.StringVar(&dump, "dump", "", "把执行结果 dump 到此 JSON 文件")
	flag.StringVar(&dataDir, "data", "agent-lab/data/products", "products.json 所在目录")
	flag.Parse()

	if goal == "" {
		fmt.Fprintln(os.Stderr, "用法: -m \"<目标>\"")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[multi] %s\n", cfg.String())

	client := llm.NewOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout, llm.WithMaxRetries(cfg.MaxRetries))

	reg := tools.NewRegistry()
	reg.Register(tools.NewProductLookup(dataDir))
	reg.Register(tools.NewPriceFormat())
	reg.Register(tools.NewPlatformLint())
	reg.Register(tools.NewSlangCheck())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runID := agent.NextRunID()
	bus := agent.NewMessageBus(runID, nil)
	multi := agent.NewMultiAgent(client, reg, cfg.ModelChat, bus)
	multi.SetMaxRounds(maxRounds)

	fmt.Printf("[multi] run=%s goal=%s maxRounds=%d\n", runID, goal, maxRounds)

	events := make(chan agent.MultiEvent, 32)
	done := make(chan struct{})
	go func() {
		for ev := range events {
			printEvent(ev)
		}
		close(done)
	}()

	result, err := multi.Run(ctx, goal, events)
	close(events)
	<-done

	fmt.Printf("\n=== 协作完成 ===\n")
	fmt.Printf("状态: %s\n", result.Status)
	fmt.Printf("轮次: %d\n", result.Rounds)
	fmt.Printf("耗时: %s\n", time.Since(result.StartedAt).Round(time.Millisecond))
	fmt.Printf("总 token: %d\n", result.TotalTokens)

	if result.FinalDraft != "" {
		fmt.Println("\n--- 最终文案 ---")
		fmt.Println(result.FinalDraft)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[multi] %v\n", err)
	}

	if dump != "" {
		data, _ := json.MarshalIndent(result, "", "  ")
		if err := os.WriteFile(dump, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "[dump] %v\n", err)
		} else {
			fmt.Printf("\n结果已保存到 %s\n", dump)
		}
	}
}

func printEvent(ev agent.MultiEvent) {
	switch ev.Type {
	case "round_start":
		fmt.Printf("\n--- round %d ---\n", ev.Round)
	case "agent_done":
		icon := "✓"
		if ev.Error != "" {
			icon = "✗"
		}
		if ev.Approved {
			icon = "✓ approve"
		}
		fmt.Printf("  %s %s: %s (%d tokens)\n", icon, ev.Agent, truncate(ev.Output, 80), ev.Tokens)
		if len(ev.Issues) > 0 {
			fmt.Printf("    issues: %v\n", ev.Issues)
		}
		if ev.Error != "" {
			fmt.Printf("    error: %s\n", ev.Error)
		}
	case "round_end":
		fmt.Printf("  -> 不通过, 反馈: %s\n", truncate(ev.Feedback, 100))
	case "done":
		if ev.Error != "" {
			fmt.Printf("\n  [done] %s\n", ev.Error)
		} else {
			fmt.Printf("\n  [done] 全部通过 ✓ (%s, %d tokens)\n", ev.Elapsed, ev.TotalTokens)
		}
	case "fail":
		fmt.Printf("\n  [fail] %s\n", ev.Error)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
