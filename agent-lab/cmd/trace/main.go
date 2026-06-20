// Command trace 查询/导出 trace (M9).
//
// 用法:
//
//	go run ./agent-lab/cmd/trace list --limit 20           # 列出最近 20 条 trace
//	go run ./agent-lab/cmd/trace show <trace_id>           # 查看单条 trace + spans
//	go run ./agent-lab/cmd/trace export <trace_id> -o out.json  # 导出 JSON
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/store"
	"ai-learn-playground/agent-lab/internal/trace"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: trace list|show|export")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open store:", err)
		os.Exit(1)
	}
	defer st.Close()

	rec := trace.NewRecorder(st)
	ctx := context.Background()

	switch os.Args[1] {
	case "list":
		cmdList(ctx, rec, os.Args[2:])
	case "show":
		cmdShow(ctx, rec, os.Args[2:])
	case "export":
		cmdExport(ctx, rec, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdList(ctx context.Context, rec *trace.Recorder, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	limit := fs.Int("limit", 20, "最多列出条数")
	fs.Parse(args)
	traces, err := rec.ListTraces(ctx, *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list:", err)
		os.Exit(1)
	}
	if len(traces) == 0 {
		fmt.Println("没有 trace 记录.")
		return
	}
	fmt.Printf("%-26s %-10s %-20s %-12s %s\n", "TRACE_ID", "STATUS", "GOAL", "STARTED", "SPANS")
	fmt.Println("──────────────────────────────────────────────────────────────────────────────────────")
	for _, t := range traces {
		goal := t.Goal
		if len([]rune(goal)) > 18 {
			goal = string([]rune(goal)[:18]) + "..."
		}
		fmt.Printf("%-26s %-10s %-20s %-12s %d\n",
			t.TraceID, t.Status, goal, formatTime(t.StartedAt), len(t.Spans))
	}
	fmt.Printf("\n共 %d 条\n", len(traces))
}

func cmdShow(ctx context.Context, rec *trace.Recorder, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: trace show <trace_id>")
		os.Exit(1)
	}
	t, err := rec.GetTrace(ctx, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "show:", err)
		os.Exit(1)
	}
	fmt.Printf("Trace: %s\n", t.TraceID)
	fmt.Printf("Goal:  %s\n", t.Goal)
	fmt.Printf("Conv:  %s\n", t.ConvID)
	fmt.Printf("Status: %s\n", t.Status)
	fmt.Printf("Started: %s\n", formatTime(t.StartedAt))
	if !t.EndedAt.IsZero() {
		fmt.Printf("Ended:   %s (耗时 %s)\n", formatTime(t.EndedAt), t.EndedAt.Sub(t.StartedAt))
	}
	if len(t.Spans) == 0 {
		fmt.Println("\n没有 spans.")
		return
	}
	fmt.Printf("\n--- Spans (%d) ---\n", len(t.Spans))
	fmt.Printf("%-26s %-6s %-20s %-10s %s\n", "SPAN_ID", "KIND", "NAME", "DURATION", "TOKENS")
	fmt.Println("──────────────────────────────────────────────────────────────────────────────")
	for _, s := range t.Spans {
		dur := "running"
		if !s.EndedAt.IsZero() {
			dur = s.Duration().String()
		}
		tokens := fmt.Sprintf("%d/%d", s.TokensIn, s.TokensOut)
		name := s.Name
		if len([]rune(name)) > 18 {
			name = string([]rune(name)[:18]) + "..."
		}
		fmt.Printf("%-26s %-6s %-20s %-10s %s\n", s.SpanID, s.Kind, name, dur, tokens)
		if s.Error != "" {
			fmt.Printf("  error: %s\n", s.Error)
		}
	}
}

func cmdExport(ctx context.Context, rec *trace.Recorder, args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	output := fs.String("o", "", "输出文件 (默认 stdout)")
	fs.Parse(args)
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "用法: trace export <trace_id> -o out.json")
		os.Exit(1)
	}
	t, err := rec.GetTrace(ctx, fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "export:", err)
		os.Exit(1)
	}
	data, _ := json.MarshalIndent(t, "", "  ")
	if *output == "" {
		fmt.Println(string(data))
	} else {
		if err := os.WriteFile(*output, data, 0644); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}
		fmt.Printf("已导出到 %s\n", *output)
	}
}

func formatTime(t interface{ Unix() int64 }) string {
	return fmt.Sprintf("%d", t.Unix())
}
