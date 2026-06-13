// Command web 是 agent-lab 的 Web UI 入口 (M0).
//
// 用法:
//
//	export OPENAI_BASE_URL=http://127.0.0.1:8080/v1
//	export OPENAI_API_KEY=sk-local
//	export AGENTLAB_PROFILE=L
//	go run ./agent-lab/cmd/web
//
// 默认 listen 127.0.0.1:8090. 用 -addr 覆盖.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/web"
)

func main() {
	var addr string
	flag.StringVar(&addr, "addr", "127.0.0.1:8090", "listen address")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[agent-lab] %s\n", cfg.String())

	client := llm.NewOpenAIClient(
		cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout,
		llm.WithMaxRetries(cfg.MaxRetries),
	)
	srv, err := web.NewServer(cfg, client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init server:", err)
		os.Exit(1)
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(os.Stderr, "[web] listening on http://%s\n", addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}
