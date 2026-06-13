// Package web 装配 cmd/web 进程的 HTTP server.
//
// 设计原则见 agent-lab/docs/06-ui.md:
//   - 标准库 net/http + html/template + embed.FS, 不引第三方框架.
//   - SSE 把 llm.Client.ChatStream 的 chunk 直接转给浏览器.
//   - 路由表与各里程碑增量见 06-ui.md.
package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"ai-learn-playground/agent-lab/internal/config"
	"ai-learn-playground/agent-lab/internal/llm"
)

// Server 是 web 模块的入口对象.
//
// 字段在后续里程碑会扩展:
//   - M1: convStore (会话内存/SQLite)
//   - M2/M3: registry (工具)
//   - M4: store (SQLite)
//   - M8: hitl
//   - M9: trace
type Server struct {
	cfg    config.Config
	llm    llm.Client
	pages  map[string]*template.Template
	static http.FileSystem
}

// NewServer 装配模板与静态资源, 返回可挂载的 Server.
func NewServer(cfg config.Config, client llm.Client) (*Server, error) {
	pages, err := loadTemplates()
	if err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("static sub fs: %w", err)
	}
	return &Server{
		cfg:    cfg,
		llm:    client,
		pages:  pages,
		static: http.FS(sub),
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
		_, _ = w.Write([]byte(`{"ok":true}`))
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

	// 占位面板 (各里程碑陆续替换).
	for _, p := range placeholders() {
		p := p
		mux.HandleFunc(p.Path, func(w http.ResponseWriter, r *http.Request) {
			s.renderPlaceholder(w, p)
		})
	}

	return logging(mux)
}

// loadTemplates 为每个 page 单独构造一棵 template tree (layout + page).
//
// html/template 的 {{define "content"}} 在同一棵 tree 里只能存活一份;
// 我们故意每个 page 都用同名块, 因此必须按 page 隔离 tree.
func loadTemplates() (map[string]*template.Template, error) {
	funcs := template.FuncMap{"navItems": navItems}

	pages := []string{"chat.html", "placeholder.html", "settings.html"}
	out := make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		t := template.New(p).Funcs(funcs)
		t, err := t.ParseFS(templatesFS, "templates/layout.html", "templates/"+p)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
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
		fmt.Printf("[web] template %s error: %v\n", page, err)
	}
}

// logging 是最简日志中间件, 后续 M9 会被 trace 替代.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush 透传给底层 ResponseWriter, 让 SSE handler 看到 Flusher.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func logging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		h.ServeHTTP(rec, r)
		// 标准 log 包足够; 控制台一行一条.
		fmt.Printf("[web] %s %s -> %d\n", r.Method, r.URL.Path, rec.status)
	})
}
