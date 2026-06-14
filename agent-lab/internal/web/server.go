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
)

// Server 是 web 模块的入口对象.
type Server struct {
	cfg       config.Config
	llm       llm.Client
	pages     map[string]*template.Template
	static    http.FileSystem
	convs     *ConversationStore
	defaultSystem string
	budget    int
	reserve   int
}

// NewServer 装配模板与静态资源, 返回可挂载的 Server.
func NewServer(cfg config.Config, client llm.Client) (*Server, error) {
	pages, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	p := prompt.Default()
	return &Server{
		cfg:           cfg,
		llm:           client,
		pages:         pages,
		static:        http.FS(sub),
		convs:         NewConversationStore(),
		defaultSystem: p.SystemPrompt,
		budget:        2048,
		reserve:       512,
	}, nil
}

// Routes 返回挂载好的 http.Handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// 静态资源.
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(s.static)))

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

	// 教程页.
	mux.HandleFunc("/tutorial", s.handleTutorial)

	// 占位面板 (各里程碑陆续替换).
	for _, p := range placeholders() {
		p := p
		mux.HandleFunc(p.Path, func(w http.ResponseWriter, r *http.Request) {
			s.renderPlaceholder(w, p)
		})
	}

	return logging(mux)
}

// loadTemplates 为每个 page 单独构造一棵 template tree.
func loadTemplates() (map[string]*template.Template, error) {
	funcs := template.FuncMap{"navItems": navItems}
	pages := []string{"chat.html", "placeholder.html", "settings.html"}
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
