package capstone

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-learn-playground/agent-lab/internal/agent"
	"ai-learn-playground/agent-lab/internal/eval"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// PipelineInput 是 Capstone 流水线的输入参数.
type PipelineInput struct {
	Seller    string   `json:"seller"`     // 卖家 ID (M4 记忆 namespace)
	SKUID     string   `json:"sku_id"`     // 商品 SKU
	Platforms []string `json:"platforms"`  // 目标平台, 例如 ["shopee","xhs"]
	Style     string   `json:"style"`      // 风格: girlfriend/promo/pro/gift
	MaxRounds int      `json:"max_rounds"` // Multi-Agent 最大轮次
}

// PipelineResult 是 Capstone 流水线的完整输出.
type PipelineResult struct {
	Input       PipelineInput         `json:"input"`
	Outputs     []PlatformOutput      `json:"outputs"`
	MultiRun    *agent.MultiRunResult `json:"multi_run"`
	EvalResults []EvalResult          `json:"eval_results"`
	EvalSummary EvalSummary           `json:"eval_summary"`
	StartedAt   time.Time             `json:"started_at"`
	FinishedAt  time.Time             `json:"finished_at"`
	Duration    string                `json:"duration"`
	Status      string                `json:"status"` // "ok" / "fail"
	Error       string                `json:"error,omitempty"`
}

// EvalResult 是单个平台输出的评测结果.
type EvalResult struct {
	Platform     string  `json:"platform"`
	JudgeScore   float64 `json:"judge_score"`
	SlangHit     float64 `json:"slang_hit"`
	ComplianceOK bool    `json:"compliance_ok"`
}

// EvalSummary 是评测汇总.
type EvalSummary struct {
	MeanJudgeScore float64 `json:"mean_judge_score"`
	MeanSlangHit   float64 `json:"mean_slang_hit"`
	ComplianceRate float64 `json:"compliance_rate"`
}

// Pipeline 是 Capstone 流水线 (M11).
type Pipeline struct {
	client  llm.Client
	reg     *tools.Registry
	model   string
	judge   *eval.Judge
	metrics *eval.Metrics
}

// NewPipeline 构造一个 Capstone 流水线.
func NewPipeline(client llm.Client, reg *tools.Registry, model string) *Pipeline {
	return &Pipeline{
		client:  client,
		reg:     reg,
		model:   model,
		judge:   eval.NewJudge(client, model),
		metrics: eval.NewMetrics(),
	}
}

// Run 执行完整流水线: Multi-Agent 协作 → 多平台输出 → 评测.
func (p *Pipeline) Run(ctx context.Context, in PipelineInput) (*PipelineResult, error) {
	start := time.Now()
	result := &PipelineResult{
		Input:     in,
		StartedAt: start,
		Status:    "running",
	}

	if in.MaxRounds <= 0 {
		in.MaxRounds = 3
	}
	if len(in.Platforms) == 0 {
		in.Platforms = []string{"shopee"}
	}
	if in.Style == "" {
		in.Style = "girlfriend"
	}

	// 1. 为每个平台运行 Multi-Agent.
	for _, platform := range in.Platforms {
		goal := buildGoal(in, platform)
		runID := agent.NextRunID()
		bus := agent.NewMessageBus(runID, nil)
		multi := agent.NewMultiAgent(p.client, p.reg, p.model, bus)
		multi.SetMaxRounds(in.MaxRounds)

		multiResult, err := multi.Run(ctx, goal, nil)
		if err != nil {
			result.Status = "fail"
			result.Error = fmt.Sprintf("platform %s: %s", platform, err)
			result.FinishedAt = time.Now()
			result.Duration = time.Since(start).Round(time.Millisecond).String()
			return result, err
		}

		result.MultiRun = multiResult
		output := ParseOutput(platform, multiResult.FinalDraft)
		result.Outputs = append(result.Outputs, output)

		// 2. 评测单个平台输出.
		ev := p.evaluate(ctx, platform, in.SKUID, output.Body)
		result.EvalResults = append(result.EvalResults, ev)
	}

	// 3. 汇总评测.
	p.summarizeEval(result)

	result.Status = "ok"
	result.FinishedAt = time.Now()
	result.Duration = time.Since(start).Round(time.Millisecond).String()
	return result, nil
}

func (p *Pipeline) evaluate(ctx context.Context, platform, skuid, output string) EvalResult {
	ev := EvalResult{Platform: platform}
	ev.SlangHit = p.metrics.SlangHit(output)
	ev.ComplianceOK = p.metrics.ComplianceOK(output, platform)

	prompt := fmt.Sprintf("为 %s 商品写文案", skuid)
	score, _, err := p.judge.Score(ctx, prompt, output)
	if err == nil {
		ev.JudgeScore = score
	}
	return ev
}

func (p *Pipeline) summarizeEval(result *PipelineResult) {
	n := len(result.EvalResults)
	if n == 0 {
		return
	}
	var sumJudge, sumSlang float64
	var compliant int
	for _, ev := range result.EvalResults {
		sumJudge += ev.JudgeScore
		sumSlang += ev.SlangHit
		if ev.ComplianceOK {
			compliant++
		}
	}
	result.EvalSummary = EvalSummary{
		MeanJudgeScore: sumJudge / float64(n),
		MeanSlangHit:   sumSlang / float64(n),
		ComplianceRate: float64(compliant) / float64(n),
	}
}

func buildGoal(in PipelineInput, platform string) string {
	styleHint := StylePersona(in.Style)
	return fmt.Sprintf("为 %s 在 %s 平台写一篇文案. 风格: %s",
		in.SKUID, PlatformName(platform), styleHint)
}

// RenderReport 把 PipelineResult 渲染成 markdown 报告.
func RenderReport(r *PipelineResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Capstone Report\n\n")
	fmt.Fprintf(&b, "- 时间: %s\n", r.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- 耗时: %s\n", r.Duration)
	fmt.Fprintf(&b, "- 卖家: %s\n", r.Input.Seller)
	fmt.Fprintf(&b, "- 商品: %s\n", r.Input.SKUID)
	fmt.Fprintf(&b, "- 平台: %s\n", strings.Join(r.Input.Platforms, ", "))
	fmt.Fprintf(&b, "- 风格: %s\n\n", r.Input.Style)

	if r.MultiRun != nil {
		fmt.Fprintf(&b, "## Multi-Agent 执行\n\n")
		fmt.Fprintf(&b, "- 状态: %s\n", r.MultiRun.Status)
		fmt.Fprintf(&b, "- 轮次: %d\n", r.MultiRun.Rounds)
		fmt.Fprintf(&b, "- Token: %d\n\n", r.MultiRun.TotalTokens)
	}

	fmt.Fprintf(&b, "## 文案输出\n\n")
	b.WriteString(FormatMarkdown(r.Outputs))

	fmt.Fprintf(&b, "## 评测结果\n\n")
	fmt.Fprintf(&b, "| 平台 | Judge | 黑话 | 合规 |\n")
	fmt.Fprintf(&b, "|---|---|---|---|\n")
	for _, ev := range r.EvalResults {
		fmt.Fprintf(&b, "| %s | %.1f | %.0f%% | %v |\n",
			PlatformName(ev.Platform), ev.JudgeScore, ev.SlangHit*100, ev.ComplianceOK)
	}
	fmt.Fprintf(&b, "\n**汇总**: Judge 均分 %.1f / 黑话命中率 %.0f%% / 合规率 %.0f%%\n",
		r.EvalSummary.MeanJudgeScore, r.EvalSummary.MeanSlangHit*100, r.EvalSummary.ComplianceRate*100)

	return b.String()
}
