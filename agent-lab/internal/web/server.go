// Package web 装配 cmd/web 进程的 HTTP server.
//
// 设计原则:
//   - 标准库 net/http + html/template + embed.FS, 不引第三方框架.
//   - SSE 把 llm.Client.ChatStream 的 chunk 直接转给浏览器.
package web

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"ai-learn-playground/agent-lab/internal/agent"
	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/hitl"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/prompt"
	"ai-learn-playground/agent-lab/internal/rag"
	"ai-learn-playground/agent-lab/internal/store"
	"ai-learn-playground/agent-lab/internal/tools"
	"ai-learn-playground/agent-lab/internal/trace"
)

// Server 是 web 模块的入口对象.
type Server struct {
	cfg           config.Config
	llm           llm.Client
	pages         map[string]*template.Template
	static        http.FileSystem
	convs         *ConversationStore
	tools         *tools.Registry
	toolHist      *ToolRecentBuffer
	store         *store.Store
	kv            *memory.KV
	retriever     *rag.Retriever
	executor      *agent.Executor
	planner       *agent.Planner
	multiFactory  MultiAgentFactory
	approvals     *hitl.Manager
	recorder      *trace.Recorder
	defaultSystem string
	budget        int
	reserve       int
}

// MultiAgentFactory 是创建 MultiAgent 的工厂函数 (每次 run 一个新实例).
type MultiAgentFactory func(bus *agent.MessageBus) *agent.MultiAgent

// ServerOption 用于自定义 NewServer 行为.
type ServerOption func(*Server)

// WithToolRegistry 注入一个 tools.Registry, 启用 /tools 面板.
func WithToolRegistry(reg *tools.Registry) ServerOption {
	return func(s *Server) {
		s.tools = reg
	}
}

// WithStore 注入 SQLite 持久层, 启用会话持久化 (write-through + 启动 hydrate).
func WithStore(st *store.Store) ServerOption {
	return func(s *Server) {
		s.store = st
	}
}

// WithMemory 注入长期记忆 KV, 启用 /memory 面板与 memory_get/memory_put 工具.
func WithMemory(kv *memory.KV) ServerOption {
	return func(s *Server) {
		s.kv = kv
	}
}

// WithRetriever 注入 RAG retriever, 启用 /knowledge 面板与 kb_search 工具.
func WithRetriever(r *rag.Retriever) ServerOption {
	return func(s *Server) {
		s.retriever = r
	}
}

// WithPlannerExecutor 注入 Planner + Executor, 启用 /plan 面板.
func WithPlannerExecutor(planner *agent.Planner, executor *agent.Executor) ServerOption {
	return func(s *Server) {
		s.planner = planner
		s.executor = executor
	}
}

// WithMultiAgent 注入 MultiAgent 工厂, 启用 /multi 面板.
func WithMultiAgent(factory MultiAgentFactory) ServerOption {
	return func(s *Server) {
		s.multiFactory = factory
	}
}

// WithApprovals 注入 HITL 审批管理器, 启用 /approvals 面板.
func WithApprovals(mgr *hitl.Manager) ServerOption {
	return func(s *Server) {
		s.approvals = mgr
	}
}

// WithTracer 注入 trace recorder, 启用 /traces 面板.
func WithTracer(rec *trace.Recorder) ServerOption {
	return func(s *Server) {
		s.recorder = rec
	}
}

// NewServer 装配模板与静态资源, 返回可挂载的 Server.
func NewServer(cfg config.Config, client llm.Client, opts ...ServerOption) (*Server, error) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	p := prompt.Default()
	s := &Server{
		cfg:           cfg,
		llm:           client,
		static:        http.FS(sub),
		convs:         NewConversationStore(),
		toolHist:      NewToolRecentBuffer(50),
		defaultSystem: p.SystemPrompt,
		budget:        2048,
		reserve:       512,
	}
	for _, opt := range opts {
		opt(s)
	}
	// 注入 store 后: 让会话 store 走 write-through, 并把磁盘上的历史会话拉回内存.
	if s.store != nil {
		s.convs.EnablePersistence(s.store)
		if err := s.hydrateConversations(); err != nil {
			fmt.Printf("[web] hydrate conversations: %v\n", err)
		}
	}
	pages, err := loadTemplates(s.enabledNav())
	if err != nil {
		return nil, err
	}
	s.pages = pages
	return s, nil
}

// hydrateConversations 把 agent.db 里的会话全部加载回内存, 实现 "重启不丢历史".
func (s *Server) hydrateConversations() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.store.ListConversations(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		_, msgs, err := s.store.LoadConversation(ctx, row.ID)
		if err != nil {
			fmt.Printf("[web] load conversation %s: %v\n", row.ID, err)
			continue
		}
		s.convs.Restore(row.ID, row.SellerID, row.Title, row.System,
			time.Unix(row.UpdatedAt, 0), msgs, s.budget, s.reserve)
	}
	if len(rows) > 0 {
		fmt.Printf("[web] hydrated %d conversations from agent.db\n", len(rows))
	}
	return nil
}

// enabledNav 返回当前 server 启用了哪些非占位面板, 给模板 navItems 用.
func (s *Server) enabledNav() map[string]bool {
	enabled := map[string]bool{
		"/chat":     true,
		"/settings": true,
		"/tutorial": true,
	}
	if s.tools != nil {
		enabled["/tools"] = true
	}
	if s.kv != nil {
		enabled["/memory"] = true
	}
	if s.retriever != nil {
		enabled["/knowledge"] = true
	}
	if s.executor != nil {
		enabled["/plan"] = true
	}
	if s.multiFactory != nil {
		enabled["/multi"] = true
	}
	if s.approvals != nil {
		enabled["/approvals"] = true
	}
	if s.recorder != nil {
		enabled["/traces"] = true
	}
	return enabled
}

// Routes 返回挂载好的 http.Handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// 静态资源. 开发期禁用浏览器缓存, 否则改了 css/js 强刷也常常拿到旧版.
	mux.Handle("/static/", http.StripPrefix("/static/", noCache(http.FileServer(s.static))))

	// 健康检查.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"milestone":"M4"}`))
	})

	// 入口重定向.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			s.renderNotFound(w, r)
			return
		}
		http.Redirect(w, r, "/chat", http.StatusFound)
	})

	// Chat 面板与 API.
	mux.HandleFunc("/chat", s.handleChatPage)
	mux.HandleFunc("/api/chat", s.handleChatAPI)
	mux.HandleFunc("/api/conversations", s.handleConversationsAPI)

	// Tools 面板 (M2): 仅在注入 Registry 时启用.
	if s.tools != nil {
		mux.HandleFunc("/tools", s.handleToolsPage)
		mux.HandleFunc("/api/tools/recent", s.handleToolsRecent)
		mux.HandleFunc("/api/tools/invoke", s.handleToolsInvoke)
	}

	// Memory 面板 (M4): 仅在注入 KV 时启用.
	if s.kv != nil {
		mux.HandleFunc("/memory", s.handleMemoryPage)
		mux.HandleFunc("/api/memory", s.handleMemoryAPI)
	}

	// Knowledge 面板 (M5): 仅在注入 Retriever 时启用.
	if s.retriever != nil {
		mux.HandleFunc("/knowledge", s.handleKnowledgePage)
		mux.HandleFunc("/api/knowledge", s.handleKnowledgeAPI)
	}

	// Plan 面板 (M6): 仅在注入 Executor 时启用.
	if s.executor != nil {
		mux.HandleFunc("/plan", s.handlePlanPage)
		mux.HandleFunc("/api/plan/generate", s.handlePlanGenerate)
		mux.HandleFunc("/api/plan/execute", s.handlePlanExecute)
	}

	// Multi-Agent 面板 (M7): 仅在注入 MultiAgent 工厂时启用.
	if s.multiFactory != nil {
		mux.HandleFunc("/multi", s.handleMultiPage)
		mux.HandleFunc("/api/multi/run", s.handleMultiRun)
	}

	// Approvals 面板 (M8): 仅在注入审批管理器时启用.
	if s.approvals != nil {
		mux.HandleFunc("/approvals", s.handleApprovalsPage)
		mux.HandleFunc("/api/approvals", s.handleApprovalsAPI)
	}

	// Traces 面板 (M9): 仅在注入 recorder 时启用.
	if s.recorder != nil {
		mux.HandleFunc("/traces", s.handleTracesPage)
		mux.HandleFunc("/api/traces", s.handleTracesAPI)
	}

	// 教程页.
	mux.HandleFunc("/tutorial", s.handleTutorial)

	// 占位面板 (各里程碑陆续替换).
	for _, p := range placeholders() {
		p := p
		// 已实装的面板跳过占位注册.
		if p.Path == "/tools" && s.tools != nil {
			continue
		}
		if p.Path == "/memory" && s.kv != nil {
			continue
		}
		if p.Path == "/knowledge" && s.retriever != nil {
			continue
		}
		if p.Path == "/plan" && s.executor != nil {
			continue
		}
		if p.Path == "/multi" && s.multiFactory != nil {
			continue
		}
		if p.Path == "/approvals" && s.approvals != nil {
			continue
		}
		if p.Path == "/traces" && s.recorder != nil {
			continue
		}
		mux.HandleFunc(p.Path, func(w http.ResponseWriter, r *http.Request) {
			s.renderPlaceholder(w, p)
		})
	}

	return logging(mux)
}

// loadTemplates 为每个 page 单独构造一棵 template tree.
func loadTemplates(enabled map[string]bool) (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"navItems": func(active string) []NavItem { return navItems(active, enabled) },
	}
	pages := []string{"chat.html", "placeholder.html", "settings.html", "tools.html", "memory.html", "knowledge.html", "plan.html", "multi.html", "approvals.html", "traces.html"}
	out := make(map[string]*template.Template, len(pages))
	var err error
	for _, p := range pages {
		t := template.New(p).Funcs(funcs)
		t, err = t.ParseFS(templatesFS, "templates/layout.html", "templates/"+p)
		if err != nil {
			return nil, err
		}
		out[p] = t
	}
	return out, nil
}

// renderPage 在指定 page 的 tree 上执行 "layout".
func (s *Server) renderPage(w http.ResponseWriter, page string, data any) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "unknown page: "+page, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// logging 是最简日志中间件.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func logging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		h.ServeHTTP(rec, r)
		fmt.Printf("[web] %s %s -> %d\n", r.Method, r.URL.Path, rec.status)
	})
}

// noCache 禁用浏览器对静态资源的缓存, 避免改 css/js 后强刷仍命中旧版.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		h.ServeHTTP(w, r)
	})
}
