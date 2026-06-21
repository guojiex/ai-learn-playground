// Command capstone 是 agent-lab 的毕业项目入口 (M11).
//
// 一条命令完成完整电商文案生成流水线:
// 输入(seller/sku/platforms/style) → Multi-Agent 协作 → 评测 → 多平台输出
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"ai-learn-playground/agent-lab/internal/capstone"
	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

func main() {
	var (
		seller    string
		skuID     string
		platforms string
		style     string
		maxRounds int
		dump      string
		dataDir   string
	)
	flag.StringVar(&seller, "seller", "A001", "卖家 ID")
	flag.StringVar(&skuID, "sku-id", "sku_001", "商品 SKU")
	flag.StringVar(&platforms, "platforms", "shopee,xhs", "目标平台 (逗号分隔)")
	flag.StringVar(&style, "style", "girlfriend", "风格: girlfriend/promo/pro/gift")
	flag.IntVar(&maxRounds, "rounds", 3, "Multi-Agent 最大轮次")
	flag.StringVar(&dump, "dump", "", "把完整结果 dump 到 JSON 文件")
	flag.StringVar(&dataDir, "data", "agent-lab/data/products", "products.json 所在目录")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[capstone] %s\n", cfg.String())

	client := llm.NewOpenAIClient(cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout, llm.WithMaxRetries(cfg.MaxRetries))
	reg := tools.NewRegistry()
	reg.Register(tools.NewProductLookup(dataDir))
	reg.Register(tools.NewPriceFormat())
	reg.Register(tools.NewPlatformLint())
	reg.Register(tools.NewSlangCheck())

	platformList := strings.Split(platforms, ",")
	for i := range platformList {
		platformList[i] = strings.TrimSpace(platformList[i])
	}

	input := capstone.PipelineInput{
		Seller:    seller,
		SKUID:     skuID,
		Platforms: platformList,
		Style:     style,
		MaxRounds: maxRounds,
	}

	fmt.Printf("[capstone] seller=%s sku=%s platforms=%v style=%s\n",
		input.Seller, input.SKUID, input.Platforms, input.Style)

	pipeline := capstone.NewPipeline(client, reg, cfg.ModelChat)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := pipeline.Run(ctx, input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[capstone] 失败: %v\n", err)
	}

	fmt.Printf("\n%s\n", capstone.RenderReport(result))

	if result.MultiRun != nil {
		fmt.Printf("[trace] %d steps, %s, %d tokens\n",
			len(result.MultiRun.Results), result.Duration, result.MultiRun.TotalTokens)
	}
	fmt.Printf("[eval]  judge=%.1f/5  slang=%.0f%%  compliance=%.0f%%\n",
		result.EvalSummary.MeanJudgeScore, result.EvalSummary.MeanSlangHit*100, result.EvalSummary.ComplianceRate*100)

	if dump != "" {
		data, _ := json.MarshalIndent(result, "", "  ")
		if err := os.WriteFile(dump, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "[dump] %v\n", err)
		} else {
			fmt.Printf("\n结果已保存到 %s\n", dump)
		}
	}
}
