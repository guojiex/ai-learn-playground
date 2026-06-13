// Command chat 是 M0 的最小可运行入口.
//
// 用法:
//
//	export OPENAI_BASE_URL=http://127.0.0.1:8080/v1
//	export OPENAI_API_KEY=sk-local
//	export AGENTLAB_PROFILE=L
//	go run ./agent-lab/cmd/chat -m "你好"
//
// 默认走流式输出. 使用 --no-stream 切换非流式.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
)

func main() {
	var (
		message     string
		systemMsg   string
		modelOver   string
		temperature float64
		maxTokens   int
		noStream    bool
	)
	flag.StringVar(&message, "m", "", "user message (required)")
	flag.StringVar(&systemMsg, "system", "你是一个稳重务实的助理.", "system prompt")
	flag.StringVar(&modelOver, "model", "", "override AGENTLAB_MODEL_CHAT")
	flag.Float64Var(&temperature, "temperature", 0.2, "sampling temperature")
	flag.IntVar(&maxTokens, "max-tokens", 512, "max output tokens")
	flag.BoolVar(&noStream, "no-stream", false, "disable streaming output")
	flag.Parse()

	if strings.TrimSpace(message) == "" {
		fmt.Fprintln(os.Stderr, "error: -m is required")
		flag.Usage()
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
	fmt.Fprintf(os.Stderr, "[agent-lab] %s\n", cfg.String())
	fmt.Fprintf(os.Stderr, "[agent-lab] model=%s stream=%v\n", model, !noStream)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := llm.NewOpenAIClient(
		cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout,
		llm.WithMaxRetries(cfg.MaxRetries),
	)

	temp := float32(temperature)
	mt := maxTokens
	req := llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemMsg},
			{Role: llm.RoleUser, Content: message},
		},
		Temperature: &temp,
		MaxTokens:   &mt,
	}

	if noStream {
		runOnce(ctx, client, req)
	} else {
		runStream(ctx, client, req)
	}
}

func runOnce(ctx context.Context, client *llm.OpenAIClient, req llm.ChatRequest) {
	resp, err := client.Chat(ctx, req)
	if err != nil {
		exitErr(err)
	}
	fmt.Println(resp.Message.Content)
	fmt.Fprintf(os.Stderr, "\n[usage] prompt=%d completion=%d total=%d finish=%s\n",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, resp.FinishReason)
}

func runStream(ctx context.Context, client *llm.OpenAIClient, req llm.ChatRequest) {
	ch, err := client.ChatStream(ctx, req)
	if err != nil {
		exitErr(err)
	}
	var (
		finish string
		usage  *llm.Usage
	)
	for chunk := range ch {
		if chunk.Err != nil {
			fmt.Fprintln(os.Stderr, "\n[stream error]", chunk.Err)
			os.Exit(1)
		}
		if chunk.DeltaContent != "" {
			fmt.Print(chunk.DeltaContent)
		}
		if chunk.FinishReason != "" {
			finish = chunk.FinishReason
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}
	fmt.Println()
	if usage != nil {
		fmt.Fprintf(os.Stderr, "[usage] prompt=%d completion=%d total=%d finish=%s\n",
			usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, finish)
	} else if finish != "" {
		fmt.Fprintf(os.Stderr, "[finish] %s\n", finish)
	}
}

func exitErr(err error) {
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "\n[interrupted]")
		os.Exit(130)
	}
	var apiErr *llm.APIError
	if errors.As(err, &apiErr) {
		fmt.Fprintln(os.Stderr, "api error:", apiErr.Error())
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
