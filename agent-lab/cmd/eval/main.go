// Command eval 跑评测集, 输出 markdown 报告 (M9).
//
// 用法:
//
//	export OPENAI_BASE_URL=http://127.0.0.1:18080/v1
//	export OPENAI_API_KEY=sk-local
//	go run ./agent-lab/cmd/eval -suite ecom-v1 -tag baseline
//	# report -> docs/reports/eval_<date>_baseline.md
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/eval"
	"ai-learn-playground/agent-lab/internal/llm"
)

func main() {
	var (
		suite      string
		tag        string
		prompts    string
		rubricPath string
		judgeModel string
		output     string
	)
	flag.StringVar(&suite, "suite", "ecom-v1", "评测集名称")
	flag.StringVar(&tag, "tag", "baseline", "本次评测标签 (用于对比)")
	flag.StringVar(&prompts, "prompts", "agent-lab/data/eval/prompts.jsonl", "评测用例 JSONL")
	flag.StringVar(&rubricPath, "rubric", "agent-lab/data/eval/judge_rubric.md", "judge rubric")
	flag.StringVar(&judgeModel, "judge-model", "", "judge 模型 (默认与被测模型相同)")
	flag.StringVar(&output, "o", "", "报告输出路径 (默认 docs/reports/eval_<date>_<tag>.md)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[eval] %s\n", cfg.String())

	if judgeModel == "" {
		judgeModel = cfg.ModelChat
	}

	cases, err := eval.LoadCases(prompts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load cases:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[eval] loaded %d cases from %s\n", len(cases), prompts)

	client := llm.NewOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout, llm.WithMaxRetries(cfg.MaxRetries))
	runner := eval.NewRunner(client, cfg.ModelChat, judgeModel)

	if rubric, err := eval.LoadRubric(rubricPath); err == nil {
		runner.Judge().WithRubric(rubric)
		fmt.Fprintf(os.Stderr, "[eval] loaded rubric from %s\n", rubricPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fmt.Fprintf(os.Stderr, "[eval] running %d cases (suite=%s, tag=%s)...\n", len(cases), suite, tag)
	report, err := runner.Run(ctx, cases, suite, tag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}

	md := eval.RenderMarkdown(report)
	if output == "" {
		dir := "docs/reports"
		_ = os.MkdirAll(dir, 0755)
		output = filepath.Join(dir, fmt.Sprintf("eval_%s_%s.md", time.Now().Format("2006-01-02"), tag))
	}
	if err := os.WriteFile(output, []byte(md), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "write report:", err)
		os.Exit(1)
	}

	fmt.Printf("\n%s\n", md)
	fmt.Printf("\n报告已保存到 %s\n", output)
}
