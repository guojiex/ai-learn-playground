// Package eval 提供 agent 级评测 (M9).
//
// 三层评测:
//   - runner: 跑固定 prompts → agent 输出 → 收集 metrics.
//   - judge: LLM-as-Judge, 用 rubric 给输出打 1-5 分.
//   - metrics: 业务度量 (黑话密度 / 平台合规 / token / 时延).
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"ai-learn-playground/agent-lab/internal/llm"
)

// Case 是一条评测用例.
type Case struct {
	ID       string `json:"id"`
	Prompt   string `json:"prompt"`
	Platform string `json:"platform"`
	Category string `json:"category"`
	Expected string `json:"expected,omitempty"`
}

// CaseResult 是一条用例的评测结果.
type CaseResult struct {
	Case
	Output       string  `json:"output"`
	JudgeScore   float64 `json:"judge_score"`
	JudgeReason  string  `json:"judge_reason,omitempty"`
	SlangHit     float64 `json:"slang_hit"`     // 黑话命中率 0-1
	ComplianceOK bool    `json:"compliance_ok"` // 平台合规
	TokensIn     int     `json:"tokens_in"`
	TokensOut    int     `json:"tokens_out"`
	LatencyMs    int64   `json:"latency_ms"`
	Error        string  `json:"error,omitempty"`
}

// Report 是一次评测的汇总报告.
type Report struct {
	Suite          string       `json:"suite"`
	Tag            string       `json:"tag"`
	Cases          []CaseResult `json:"cases"`
	StartedAt      time.Time    `json:"started_at"`
	MeanJudgeScore float64      `json:"mean_judge_score"`
	MeanSlangHit   float64      `json:"mean_slang_hit"`
	ComplianceRate float64      `json:"compliance_rate"`
	MeanTokens     float64      `json:"mean_tokens"`
	MeanLatencyMs  float64      `json:"mean_latency_ms"`
	OKCount        int          `json:"ok_count"`
	FailCount      int          `json:"fail_count"`
}

// Runner 跑评测集.
type Runner struct {
	client  llm.Client
	model   string
	judge   *Judge
	metrics *Metrics
}

// NewRunner 构造一个评测 runner.
func NewRunner(client llm.Client, model, judgeModel string) *Runner {
	return &Runner{
		client:  client,
		model:   model,
		judge:   NewJudge(client, judgeModel),
		metrics: NewMetrics(),
	}
}

// Judge 返回内部的 judge, 供外部配置 rubric.
func (r *Runner) Judge() *Judge {
	return r.judge
}

// Run 跑全部 cases, 返回 report.
func (r *Runner) Run(ctx context.Context, cases []Case, suite, tag string) (*Report, error) {
	report := &Report{
		Suite:     suite,
		Tag:       tag,
		StartedAt: time.Now(),
	}
	for _, c := range cases {
		res := r.runCase(ctx, c)
		report.Cases = append(report.Cases, res)
		if res.Error != "" {
			report.FailCount++
		} else {
			report.OKCount++
		}
	}
	r.summarize(report)
	return report, nil
}

func (r *Runner) runCase(ctx context.Context, c Case) CaseResult {
	res := CaseResult{Case: c}
	start := time.Now()
	system := fmt.Sprintf("你是台湾电商文案专家. 平台: %s. 类目: %s. 根据用户需求生成文案, 只输出文案正文.", c.Platform, c.Category)
	resp, err := r.client.Chat(ctx, llm.ChatRequest{
		Model: r.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: system},
			{Role: llm.RoleUser, Content: c.Prompt},
		},
		Temperature: float32Ptr(0.6),
		MaxTokens:   intPtr(512),
	})
	res.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.Output = strings.TrimSpace(resp.Message.Content)
	res.TokensIn = resp.Usage.PromptTokens
	res.TokensOut = resp.Usage.CompletionTokens

	// Judge 评分.
	score, reason, err := r.judge.Score(ctx, c.Prompt, res.Output)
	if err == nil {
		res.JudgeScore = score
		res.JudgeReason = reason
	}

	// 业务度量.
	res.SlangHit = r.metrics.SlangHit(res.Output)
	res.ComplianceOK = r.metrics.ComplianceOK(res.Output, c.Platform)
	return res
}

func (r *Runner) summarize(report *Report) {
	n := len(report.Cases)
	if n == 0 {
		return
	}
	var sumJudge, sumSlang, sumTokens, sumLatency float64
	var compliant int
	for _, c := range report.Cases {
		sumJudge += c.JudgeScore
		sumSlang += c.SlangHit
		sumTokens += float64(c.TokensIn + c.TokensOut)
		sumLatency += float64(c.LatencyMs)
		if c.ComplianceOK {
			compliant++
		}
	}
	report.MeanJudgeScore = sumJudge / float64(n)
	report.MeanSlangHit = sumSlang / float64(n)
	report.ComplianceRate = float64(compliant) / float64(n)
	report.MeanTokens = sumTokens / float64(n)
	report.MeanLatencyMs = sumLatency / float64(n)
}

// LoadCases 从 JSONL 文件加载评测用例.
func LoadCases(path string) ([]Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cases: %w", err)
	}
	var cases []Case
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		cases = append(cases, c)
	}
	return cases, nil
}

// RenderMarkdown 把 report 渲染成 markdown 表格.
func RenderMarkdown(r *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Eval Report: %s (%s)\n\n", r.Suite, r.Tag)
	fmt.Fprintf(&b, "- 时间: %s\n", r.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- 用例: %d (ok=%d, fail=%d)\n", len(r.Cases), r.OKCount, r.FailCount)
	fmt.Fprintf(&b, "- Judge 均分: %.2f / 5\n", r.MeanJudgeScore)
	fmt.Fprintf(&b, "- 黑话命中率: %.1f%%\n", r.MeanSlangHit*100)
	fmt.Fprintf(&b, "- 合规率: %.1f%%\n", r.ComplianceRate*100)
	fmt.Fprintf(&b, "- 平均 token: %.0f\n", r.MeanTokens)
	fmt.Fprintf(&b, "- 平均时延: %.0fms\n\n", r.MeanLatencyMs)

	b.WriteString("| ID | 平台 | Judge | 黑话 | 合规 | token | 时延 | 错误 |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cases {
		errCell := ""
		if c.Error != "" {
			errCell = c.Error
		}
		fmt.Fprintf(&b, "| %s | %s | %.1f | %.0f%% | %v | %d | %dms | %s |\n",
			c.ID, c.Platform, c.JudgeScore, c.SlangHit*100, c.ComplianceOK,
			c.TokensIn+c.TokensOut, c.LatencyMs, errCell)
	}
	return b.String()
}

func float32Ptr(v float32) *float32 { return &v }
func intPtr(v int) *int             { return &v }
