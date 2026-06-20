// Command hitl 是 agent-lab 的审批管理 CLI (M8).
//
// 用法:
//
//	go run ./agent-lab/cmd/hitl list                    # 列出 pending 审批
//	go run ./agent-lab/cmd/hitl list --all               # 列出全部审批
//	go run ./agent-lab/cmd/hitl show <id>                # 查看详情
//	go run ./agent-lab/cmd/hitl approve <id> --note "ok"  # 批准
//	go run ./agent-lab/cmd/hitl reject <id> --note "no"   # 拒绝
//	go run ./agent-lab/cmd/hitl edit <id> --args '{}'     # 修改参数后批准
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/hitl"
	"ai-learn-playground/agent-lab/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: hitl list|show|approve|reject|edit")
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

	mgr := hitl.NewManager(st)
	ctx := context.Background()

	switch os.Args[1] {
	case "list":
		cmdList(ctx, mgr, os.Args[2:])
	case "show":
		cmdShow(ctx, mgr, os.Args[2:])
	case "approve":
		cmdApprove(ctx, mgr, os.Args[2:])
	case "reject":
		cmdReject(ctx, mgr, os.Args[2:])
	case "edit":
		cmdEdit(ctx, mgr, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdList(ctx context.Context, mgr *hitl.Manager, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	all := fs.Bool("all", false, "列出全部审批 (含已完成)")
	limit := fs.Int("limit", 20, "最多列出条数")
	fs.Parse(args)

	if *all {
		approvals, err := mgr.ListAll(ctx, *limit)
		if err != nil {
			fmt.Fprintln(os.Stderr, "list:", err)
			os.Exit(1)
		}
		printTable(approvals)
	} else {
		approvals, err := mgr.ListPending(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "list pending:", err)
			os.Exit(1)
		}
		if len(approvals) == 0 {
			fmt.Println("没有待审批项.")
			return
		}
		printTable(approvals)
	}
}

func cmdShow(ctx context.Context, mgr *hitl.Manager, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: hitl show <id>")
		os.Exit(1)
	}
	a, err := mgr.Get(ctx, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "show:", err)
		os.Exit(1)
	}
	data, _ := json.MarshalIndent(a, "", "  ")
	fmt.Println(string(data))
}

func cmdApprove(ctx context.Context, mgr *hitl.Manager, args []string) {
	fs := flag.NewFlagSet("approve", flag.ExitOnError)
	note := fs.String("note", "", "审批备注")
	reviewer := fs.String("reviewer", "cli", "审批人")
	fs.Parse(args)
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "用法: hitl approve <id> --note \"ok\"")
		os.Exit(1)
	}
	a, err := mgr.Approve(ctx, fs.Arg(0), *reviewer, *note)
	if err != nil {
		fmt.Fprintln(os.Stderr, "approve:", err)
		os.Exit(1)
	}
	fmt.Printf("已批准: %s (tool=%s)\n", a.ID, a.Tool)
}

func cmdReject(ctx context.Context, mgr *hitl.Manager, args []string) {
	fs := flag.NewFlagSet("reject", flag.ExitOnError)
	note := fs.String("note", "", "拒绝原因")
	reviewer := fs.String("reviewer", "cli", "审批人")
	fs.Parse(args)
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "用法: hitl reject <id> --note \"原因\"")
		os.Exit(1)
	}
	a, err := mgr.Reject(ctx, fs.Arg(0), *reviewer, *note)
	if err != nil {
		fmt.Fprintln(os.Stderr, "reject:", err)
		os.Exit(1)
	}
	fmt.Printf("已拒绝: %s (tool=%s, reason=%s)\n", a.ID, a.Tool, *note)
}

func cmdEdit(ctx context.Context, mgr *hitl.Manager, args []string) {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	editedArgs := fs.String("args", "", "修改后的参数 JSON")
	note := fs.String("note", "edited", "备注")
	reviewer := fs.String("reviewer", "cli", "审批人")
	fs.Parse(args)
	if fs.NArg() < 1 || *editedArgs == "" {
		fmt.Fprintln(os.Stderr, "用法: hitl edit <id> --args '{...}'")
		os.Exit(1)
	}
	a, err := mgr.Edit(ctx, fs.Arg(0), *reviewer, *note, *editedArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "edit:", err)
		os.Exit(1)
	}
	fmt.Printf("已修改并批准: %s (tool=%s)\n", a.ID, a.Tool)
}

func printTable(approvals []hitl.Approval) {
	fmt.Printf("%-28s %-10s %-8s %-20s %-20s %s\n", "ID", "STATUS", "RISK", "TOOL", "CONV_ID", "CREATED")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────────────────────────")
	for _, a := range approvals {
		t := time.Unix(a.CreatedAt, 0).Format("01-02 15:04")
		fmt.Printf("%-28s %-10s %-8s %-20s %-20s %s\n", a.ID, a.Status, a.RiskLevel, a.Tool, a.ConvID, t)
	}
	fmt.Printf("\n共 %d 条\n", len(approvals))
}
