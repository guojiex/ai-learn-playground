package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"ai-learn-playground/agent-lab/internal/llm"
)

// Judge 用 LLM-as-Judge 给 agent 输出打分 (M9).
//
// 用一段 rubric (打分细则) 让 LLM 给输出 1-5 分.
// 同模型自评偏松, 必要时可用更大档 (XL) 做 judge.
type Judge struct {
	client   llm.Client
	model    string
	rubric   string
	maxRetry int
}

// NewJudge 构造一个 judge. judgeModel 可与被测模型不同.
func NewJudge(client llm.Client, judgeModel string) *Judge {
	return &Judge{
		client:   client,
		model:    judgeModel,
		rubric:   defaultRubric,
		maxRetry: 2,
	}
}

// WithRubric 替换默认 rubric.
func (j *Judge) WithRubric(rubric string) *Judge {
	j.rubric = rubric
	return j
}

// Score 给 output 打 1-5 分, 返回 (score, reason, error).
func (j *Judge) Score(ctx context.Context, prompt, output string) (float64, string, error) {
	if j.client == nil {
		return 0, "", fmt.Errorf("judge client is nil")
	}
	systemPrompt := fmt.Sprintf(`你是一个电商文案评审员. 按以下 rubric 给文案打 1-5 分:

%s

输出格式: 严格输出 JSON, 不要解释:
{"score": <1-5的整数>, "reason": "<一句话理由>"}`, j.rubric)

	userMsg := fmt.Sprintf("用户需求:\n%s\n\n文案:\n%s", prompt, output)

	var lastErr error
	for attempt := 0; attempt <= j.maxRetry; attempt++ {
		resp, err := j.client.Chat(ctx, llm.ChatRequest{
			Model: j.model,
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: systemPrompt},
				{Role: llm.RoleUser, Content: userMsg},
			},
			Temperature: float32Ptr(0.2),
			MaxTokens:   intPtr(256),
		})
		if err != nil {
			return 0, "", fmt.Errorf("judge chat: %w", err)
		}
		score, reason, err := parseJudgeJSON(resp.Message.Content)
		if err != nil {
			lastErr = err
			continue
		}
		return score, reason, nil
	}
	return 0, "", fmt.Errorf("judge parse failed after %d retries: %w", j.maxRetry, lastErr)
}

// parseJudgeJSON 从 LLM 输出中提取 {score, reason}.
func parseJudgeJSON(raw string) (float64, string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, "", fmt.Errorf("empty judge output")
	}
	candidates := make([]string, 0, 4)
	if strings.HasPrefix(text, "{") {
		candidates = append(candidates, text)
	}
	if s, ok := extractFenced(text, "```json", "```"); ok {
		candidates = append(candidates, s)
	}
	if s, ok := extractFenced(text, "```", "```"); ok {
		candidates = append(candidates, s)
	}
	if brace, ok := extractFirstBrace(text); ok {
		candidates = append(candidates, brace)
	}
	for _, c := range candidates {
		var m map[string]any
		if err := json.Unmarshal([]byte(c), &m); err == nil {
			score := 0.0
			if v, ok := m["score"]; ok {
				switch s := v.(type) {
				case float64:
					score = s
				case int:
					score = float64(s)
				case string:
					fmt.Sscanf(s, "%f", &score)
				}
			}
			reason, _ := m["reason"].(string)
			if score > 0 {
				return score, reason, nil
			}
		}
	}
	return 0, "", fmt.Errorf("cannot parse judge JSON: %s", text)
}

func extractFenced(text, open, close string) (string, bool) {
	start := strings.Index(text, open)
	if start < 0 {
		return "", false
	}
	start += len(open)
	end := strings.Index(text[start:], close)
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(text[start : start+end]), true
}

func extractFirstBrace(text string) (string, bool) {
	start := strings.Index(text, "{")
	if start < 0 {
		return "", false
	}
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start : i+1], true
			}
		}
	}
	return "", false
}

const defaultRubric = `1分: 文案与需求无关, 或包含违禁词/严重错误.
2分: 文案勉强相关, 但缺乏卖点, 或格式混乱.
3分: 文案基本可用, 有卖点但不够突出, 格式尚可.
4分: 文案质量好, 卖点清晰, 格式规范, 符合平台风格.
5分: 文案优秀, 卖点突出且有吸引力, 格式完美, 完全符合平台风格.`

// LoadRubric 从文件加载自定义 rubric.
func LoadRubric(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read rubric: %w", err)
	}
	return string(data), nil
}
