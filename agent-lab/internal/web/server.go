// Package web 装配 cmd/web 进程的 HTTP server.
//
// 设计原则:
//   - 标准库 net/http + html/template + embed.FS, 不引第三方框架.
//   - SSE 把 llm.Client.ChatStream 的 chunk 直接转给浏览器.
package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/prompt"
	"ai-learn-playground/agent-lab/internal/tools"
)

// Server 是 web 模块的入口对象.
type Server struct {
	cfg       config.Config
	llm       llm.Client
	pages     map[string]*template.Template
	static    http.FileSystem
	convs     *ConversationStore
	tools     *tools.Registry
	toolHist  *ToolRecentBuffer
	defaultSystem string
	budget    int
	reserve   int
}

// ServerOption 用于自定义 NewServer 行为.
type ServerOption func(*Server)

// WithToolRegistry 注入一个 tools.Registry, 启用 /tools 面板.
func WithToolRegistry(reg *tools.Registry) ServerOption {
	return func(s *Server) {
		s.tools = reg
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
	pages, err := loadTemplates(s.enabledNav())
	if err != nil {
		return nil, err
	}
	s.pages = pages
	return s, nil
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
		_, _ = w.Write([]byte(`{"ok":true,"milestone":"M1"}`))
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

	// 教程页.
	mux.HandleFunc("/tutorial", s.handleTutorial)

	// 占位面板 (各里程碑陆续替换).
	for _, p := range placeholders() {
		p := p
		// 已实装的面板跳过占位注册.
		if p.Path == "/tools" && s.tools != nil {
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
	pages := []string{"chat.html", "placeholder.html", "settings.html", "tools.html"}
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
