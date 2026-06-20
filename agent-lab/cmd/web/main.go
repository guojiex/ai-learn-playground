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
// 默认 SQLite 库为 agent-lab/data/agent.db, 用 -db 覆盖 (M4).
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

	"ai-learn-playground/agent-lab/internal/agent"
	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/hitl"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/rag"
	"ai-learn-playground/agent-lab/internal/store"
	"ai-learn-playground/agent-lab/internal/tools"
	"ai-learn-playground/agent-lab/internal/trace"
	"ai-learn-playground/agent-lab/internal/web"
)

func main() {
	var (
		addr    string
		dataDir string
		dbPath  string
	)
	flag.StringVar(&addr, "addr", "127.0.0.1:8090", "listen address")
	flag.StringVar(&dataDir, "data", "agent-lab/data/products", "tools 工具用的 products.json 所在目录")
	flag.StringVar(&dbPath, "db", "", "SQLite 路径 (默认读 AGENTLAB_DB_PATH, 再退回 agent-lab/data/agent.db); :memory: 走纯内存")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[agent-lab] %s\n", cfg.String())

	if dbPath == "" {
		dbPath = cfg.DBPath
	}

	client := llm.NewOpenAIClient(
		cfg.BaseURL, cfg.APIKey, cfg.RequestTimeout,
		llm.WithMaxRetries(cfg.MaxRetries),
	)

	// M4: 打开 SQLite 持久层 (agent.db), 失败不致命 — 退化为纯内存会话.
	var st *store.Store
	if dbPath != "" {
		st, err = store.Open(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[agent-lab] open store %s: %v (退化为内存模式)\n", dbPath, err)
			st = nil
		} else {
			fmt.Fprintf(os.Stderr, "[agent-lab] store=%s\n", dbPath)
			defer st.Close()
		}
	}

	// 长期记忆 KV 绑定到同一个 store.
	var kv *memory.KV
	if st != nil {
		kv = memory.NewKV(st)
	}

	// M5: RAG 向量库 + retriever. 需要 embedder (可指向同一 fake/真实后端).
	var retriever *rag.Retriever
	if st != nil {
		embedder := llm.NewOpenAIEmbedder(cfg.EmbedBaseURL, cfg.EmbedAPIKey, cfg.ModelEmbed, cfg.RequestTimeout, 0)
		vs, err := memory.NewVectorStore(st)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[agent-lab] vector store: %v (RAG 不可用)\n", err)
		} else {
			retriever = rag.NewRetriever(embedder, vs)
			fmt.Fprintf(os.Stderr, "[agent-lab] rag: %d chunks (dim=%d)\n", retriever.Count(), retriever.Dim())
		}
	}

	reg := tools.NewRegistry()
	reg.Register(tools.NewProductLookup(dataDir))
	reg.Register(tools.NewPriceFormat())
	reg.Register(tools.NewPlatformLint())
	reg.Register(tools.NewSlangCheck())
	if kv != nil {
		reg.Register(tools.NewMemoryGet(kv))
		reg.Register(tools.NewMemoryPut(kv))
	}
	if retriever != nil {
		reg.Register(tools.NewKBSearch(retriever))
	}
	fmt.Fprintf(os.Stderr, "[agent-lab] tools=%v\n", reg.Names())

	// M6: Planner + Executor.
	planner := agent.NewPlanner(client, reg, cfg.ModelChat)
	executor := agent.NewExecutor(planner, client, reg, cfg.ModelChat)

	// M7: Multi-Agent factory (每次 run 创建新实例, 绑定独立的 bus).
	multiFactory := func(bus *agent.MessageBus) *agent.MultiAgent {
		return agent.NewMultiAgent(client, reg, cfg.ModelChat, bus)
	}

	// M8: HITL 审批管理器.
	var approvals *hitl.Manager
	if st != nil {
		approvals = hitl.NewManager(st)
	}

	// M9: Trace recorder.
	var recorder *trace.Recorder
	if st != nil {
		recorder = trace.NewRecorder(st)
	}

	var srvOpts []web.ServerOption
	srvOpts = append(srvOpts, web.WithToolRegistry(reg))
	if st != nil {
		srvOpts = append(srvOpts, web.WithStore(st))
	}
	if kv != nil {
		srvOpts = append(srvOpts, web.WithMemory(kv))
	}
	if retriever != nil {
		srvOpts = append(srvOpts, web.WithRetriever(retriever))
	}
	srvOpts = append(srvOpts, web.WithPlannerExecutor(planner, executor))
	srvOpts = append(srvOpts, web.WithMultiAgent(multiFactory))
	if approvals != nil {
		srvOpts = append(srvOpts, web.WithApprovals(approvals))
	}
	if recorder != nil {
		srvOpts = append(srvOpts, web.WithTracer(recorder))
	}

	srv, err := web.NewServer(cfg, client, srvOpts...)
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
