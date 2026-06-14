// Command chat 是 agent-lab 的 CLI 入口, 支持多轮 REPL.
//
// 无参数默认进入 REPL. 特殊命令以冒号开头:
//
//	:reset             清空历史 (保留 system prompt)
//	:system <text>     覆盖 system prompt
//	:save <path>       把 system+历史存为 JSON
//	:load <path>       从 JSON 恢复
//	:history           打印全部历史
//	:quit / :exit      退出
//
// 用法:
//
//	export OPENAI_BASE_URL=http://127.0.0.1:8080/v1
//	export OPENAI_API_KEY=sk-local
//	export AGENTLAB_PROFILE=L
//	go run ./agent-lab/cmd/chat
package main

import (
	"bufio"
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
	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/prompt"
)

func main() {
	var (
		initialMsg string
		modelOver  string
		temperature float64
		maxTokens  int
		noStream   bool
		persona    string
		budget     int
		reserve    int
	)
	flag.StringVar(&initialMsg, "m", "", "首条消息 (发完后进入 REPL); 留空则只打印欢迎")
	flag.StringVar(&modelOver, "model", "", "覆盖 AGENTLAB_MODEL_CHAT")
	flag.Float64Var(&temperature, "temperature", 0.4, "采样温度")
	flag.IntVar(&maxTokens, "max-tokens", 512, "输出 token 上限")
	flag.BoolVar(&noStream, "no-stream", false, "关闭流式输出")
	flag.StringVar(&persona, "persona", "", "角色卡文件名 (不含 .md), 留空用默认")
	flag.IntVar(&budget, "budget", 2048, "上下文总 token 预算")
	flag.IntVar(&reserve, "reserve", 512, "输出预留 token 数")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	model := cfg.ModelChat
	if modelOver != "" {
		model = modelOver
	}

	p := prompt.Default()
	if persona != "" {
		if loaded, err := prompt.LoadPersona(persona); err == nil {
			p = loaded
		} else {
			fmt.Fprintf(os.Stderr, "[warn] 加载角色卡失败, 用默认: %v\n", err)
		}
	}

	mem := memory.NewShortTerm(p.SystemPrompt, budget, reserve)

	client := llm.NewOpenAIClient(
		cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout,
		llm.WithMaxRetries(cfg.MaxRetries),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "[agent-lab] profile=%s model=%s budget=%d reserve=%d stream=%v\n",
		cfg.Profile, model, budget, reserve, !noStream)
	fmt.Fprintln(os.Stderr, "[agent-lab] 输入 :help 查看命令. 空行退出.")

	if initialMsg != "" {
		if err := turn(ctx, client, model, mem, initialMsg, temperature, maxTokens, noStream); err != nil {
			if errors.Is(err, errAbort) {
				fmt.Fprintln(os.Stderr, "\n[interrupted]")
				return
			}
			fmt.Fprintln(os.Stderr, "error:", err)
		}
	}

	// REPL
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "\n> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ":") {
			if handled := handleCommand(line, mem); handled == handledQuit {
				return
			}
			continue
		}
		if err := turn(ctx, client, model, mem, line, temperature, maxTokens, noStream); err != nil {
			if errors.Is(err, errAbort) {
				fmt.Fprintln(os.Stderr, "\n[interrupted]")
				return
			}
			fmt.Fprintln(os.Stderr, "error:", err)
		}
	}
}

var errAbort = errors.New("aborted by user")

type handled int

const (
	handledOk handled = iota
	handledQuit
)

func handleCommand(line string, mem *memory.ShortTerm) handled {
	parts := strings.Fields(line)
	cmd := parts[0]
	rest := ""
	if len(parts) > 1 {
		rest = strings.Join(parts[1:], " ")
	}
	switch cmd {
	case ":reset":
		mem.Reset()
		fmt.Fprintln(os.Stderr, "[ok] 已清空历史")
	case ":system":
		if rest == "" {
			fmt.Fprintln(os.Stderr, "当前 system:\n", mem.System())
			return handledOk
		}
		mem.SetSystem(rest)
		fmt.Fprintln(os.Stderr, "[ok] system prompt 已更新")
	case ":save":
		if rest == "" {
			rest = "./chat-session.json"
		}
		if err := mem.SaveToFile(rest); err != nil {
			fmt.Fprintln(os.Stderr, "save error:", err)
			return handledOk
		}
		fmt.Fprintln(os.Stderr, "[ok] 已保存 →", rest)
	case ":load":
		if rest == "" {
			fmt.Fprintln(os.Stderr, "usage: :load <path>")
			return handledOk
		}
		if err := mem.LoadFromFile(rest); err != nil {
			fmt.Fprintln(os.Stderr, "load error:", err)
			return handledOk
		}
		fmt.Fprintf(os.Stderr, "[ok] 已加载 %d 轮历史\n", mem.Len())
	case ":history":
		msgs := mem.Messages()
		if len(msgs) == 0 {
			fmt.Fprintln(os.Stderr, "(空)")
			return handledOk
		}
		for i, m := range msgs {
			fmt.Fprintf(os.Stderr, "[%d] %s: %s\n", i, m.Role, truncate(m.Content, 120))
		}
	case ":help":
		fmt.Fprintln(os.Stderr, "命令: :reset | :system [text] | :save [path] | :load <path> | :history | :quit")
	case ":quit", ":exit", ":q":
		return handledQuit
	default:
		fmt.Fprintln(os.Stderr, "未知命令, 输入 :help")
	}
	return handledOk
}

// turn 处理一轮对话: append user → 压缩 → 调用 LLM → append assistant.
func turn(ctx context.Context, client *llm.OpenAIClient, model string,
	mem *memory.ShortTerm, userText string, temperature float64, maxTokens int, noStream bool) error {

	mem.Append(llm.RoleUser, userText)

	// 压缩检查
	info, cerr := mem.EnsureBudget(ctx, client, model, maxTokens)
	if cerr != nil {
		fmt.Fprintf(os.Stderr, "\n[memory] %v\n", cerr)
	}
	if info.DidCompress {
		fmt.Fprintf(os.Stderr, "\n[memory] 压缩 %d→%d 轮 (%d chars)\n",
			info.BeforeTurns, info.AfterTurns, info.BeforeChars)
	}

	msgs := mem.Snapshot()
	temp := float32(temperature)
	mt := maxTokens
	req := llm.ChatRequest{
		Model:       model,
		Messages:    msgs,
		Temperature: &temp,
		MaxTokens:   &mt,
		Stream:      !noStream,
	}

	var reply string
	if noStream {
		resp, err := client.Chat(ctx, req)
		if err != nil {
			return wrapErr(err)
		}
		reply = resp.Message.Content
		fmt.Println(reply)
		fmt.Fprintf(os.Stderr, "[usage] prompt=%d completion=%d total=%d\n",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	} else {
		ch, err := client.ChatStream(ctx, req)
		if err != nil {
			return wrapErr(err)
		}
		var sb strings.Builder
		for chunk := range ch {
			if chunk.Err != nil {
				if errors.Is(chunk.Err, context.Canceled) {
					return errAbort
				}
				return chunk.Err
			}
			if chunk.DeltaContent != "" {
				sb.WriteString(chunk.DeltaContent)
				fmt.Print(chunk.DeltaContent)
			}
			if chunk.Usage != nil {
				fmt.Fprintf(os.Stderr, "\n[usage] prompt=%d completion=%d total=%d",
					chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens, chunk.Usage.TotalTokens)
			}
		}
		fmt.Println()
		reply = sb.String()
	}
	mem.Append(llm.RoleAssistant, reply)
	return nil
}

func wrapErr(err error) error {
	var apiErr *llm.APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("api error: %w", err)
	}
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
