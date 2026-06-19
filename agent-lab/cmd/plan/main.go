// Command plan 是 agent-lab 的 Planner-Executor CLI 入口 (M6).
//
// 用法:
//
//	export OPENAI_BASE_URL=http://127.0.0.1:18080/v1
//	export OPENAI_API_KEY=sk-local
//	export AGENTLAB_PROFILE=L
//	go run ./agent-lab/cmd/plan -m "为 sku_001 在小红书台湾发一篇上新文案"
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
		goal    string
		dump    string
		dataDir string
	)
	flag.StringVar(&goal, "m", "", "目标 (必填)")
	flag.StringVar(&dump, "dump", "", "把执行轨迹 dump 到此 JSON 文件")
	flag.StringVar(&dataDir, "data", "agent-lab/data/products", "tools 工具用的 products.json 所在目录")
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
	fmt.Fprintf(os.Stderr, "[plan] %s\n", cfg.String())

	client := llm.NewOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout, llm.WithMaxRetries(cfg.MaxRetries))

	reg := tools.NewRegistry()
	reg.Register(tools.NewProductLookup(dataDir))
	reg.Register(tools.NewPriceFormat())
	reg.Register(tools.NewPlatformLint())
	reg.Register(tools.NewSlangCheck())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	planner := agent.NewPlanner(client, reg, cfg.ModelChat)
	executor := agent.NewExecutor(planner, client, reg, cfg.ModelChat)

	fmt.Printf("[planner] 生成计划中... goal=%s\n", goal)
	plan, err := planner.Plan(ctx, goal)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[planner] 失败:", err)
		os.Exit(1)
	}
	printPlan(plan)

	fmt.Printf("\n[executor] 开始执行 (%d tasks)\n", len(plan.Tasks))
	events := make(chan agent.ExecEvent, 32)
	done := make(chan struct{})
	go func() {
		for ev := range events {
			printEvent(ev)
		}
		close(done)
	}()

	run, err := executor.Execute(ctx, plan, events)
	close(events)
	<-done

	fmt.Printf("\n=== 执行完成 ===\n")
	fmt.Printf("状态: %s\n", run.Status)
	fmt.Printf("耗时: %s\n", time.Since(run.StartedAt).Round(time.Millisecond))
	fmt.Printf("重规划次数: %d\n", len(run.Replans))
	fmt.Printf("总 token: %d\n", run.TotalTokens)

	if len(run.Results) > 0 {
		fmt.Println("\n--- 任务结果 ---")
		for _, r := range run.Results {
			statusIcon := "✓"
			switch r.Status {
			case agent.TaskFail:
				statusIcon = "✗"
			case agent.TaskSkipped:
				statusIcon = "⊘"
			}
			fmt.Printf("%s %s: %s\n", statusIcon, r.TaskID, r.Status)
			if r.Output != "" {
				preview := r.Output
				if len([]rune(preview)) > 200 {
					preview = string([]rune(preview)[:200]) + "..."
				}
				fmt.Printf("  输出: %s\n", preview)
			}
			if r.Error != "" {
				fmt.Printf("  错误: %s\n", r.Error)
			}
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[executor] 最终失败: %v\n", err)
	}

	if dump != "" {
		if err := dumpRun(dump, run); err != nil {
			fmt.Fprintf(os.Stderr, "[dump] %v\n", err)
		} else {
			fmt.Printf("\n轨迹已保存到 %s\n", dump)
		}
	}
}

func printPlan(plan *agent.Plan) {
	fmt.Printf("\n=== Plan ===\n")
	fmt.Printf("目标: %s\n", plan.Goal)
	fmt.Printf("任务数: %d\n", len(plan.Tasks))
	levels := plan.TopoLevels()
	for i, level := range levels {
		fmt.Printf("  层 %d: %s\n", i+1, fmtIds(level))
	}
	for _, t := range plan.Tasks {
		kind := "tool:" + t.Tool
		if t.Agent != "" {
			kind = "agent:" + t.Agent
		}
		deps := "无"
		if len(t.Depends) > 0 {
			deps = fmtIds(t.Depends)
		}
		fmt.Printf("  %s [%s] %s (depends: %s)\n", t.ID, kind, t.Name, deps)
	}
}

func printEvent(ev agent.ExecEvent) {
	switch ev.Type {
	case "task_start":
		fmt.Printf("  [exec] %s %s ... 开始\n", ev.TaskID, ev.TaskName)
	case "task_done":
		fmt.Printf("  [exec] %s %s ... ✓ (%s)\n", ev.TaskID, ev.TaskName, ev.Elapsed)
	case "task_fail":
		fmt.Printf("  [exec] %s %s ... ✗ 失败: %s\n", ev.TaskID, ev.TaskName, ev.Error)
	case "replan":
		fmt.Printf("  [replan] %s 失败 (%s) → 生成新计划\n", ev.Replan.FailedTask, ev.Replan.Reason)
	case "plan_done":
		fmt.Printf("\n  [plan] 全部完成 ✓\n")
	case "plan_fail":
		fmt.Printf("\n  [plan] 失败: %s\n", ev.Error)
	}
}

func dumpRun(path string, run *agent.PlanRun) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func fmtIds(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += id
	}
	return "[" + out + "]"
}
