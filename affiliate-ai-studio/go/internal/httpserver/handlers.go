package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-learn-playground/affiliate-ai-studio/go/internal/app"
	"ai-learn-playground/affiliate-ai-studio/go/internal/assets"
	"ai-learn-playground/affiliate-ai-studio/go/internal/viewmodel"
)

// ViteConfig 控制 studio.html 里前端资源的注入方式。
// DevMode 为 true 时直连 Vite dev server（通过 DevServerURL），否则从 Manifest 读 build 产物。
type ViteConfig struct {
	DevMode       bool
	DevServerURL  string // 例如 "http://localhost:5173"
	EntryModule   string // 例如 "/src/main.tsx"
	Manifest      *assets.Manifest
}

type Handler struct {
	app  *app.App
	tmpl *template.Template
	vite ViteConfig
}

type RunAffiliateRequest struct {
	ProductURL        string  `json:"product_url"`
	ProductText       string  `json:"product_text"`
	Platform          string  `json:"platform"`
	Locale            string  `json:"locale"`
	Style             string  `json:"style"`
	MinCommissionRate float64 `json:"min_commission_rate"`
	EnableCompression bool    `json:"enable_compression"`
	Debug             bool    `json:"debug"`
}

func NewHandler(application *app.App, tmpl *template.Template) *Handler {
	return &Handler{app: application, tmpl: tmpl}
}

// WithVite 注入前端资源配置（可选）。未调用时 StudioPage 会回落到老 vanilla 模板行为，
// 便于纯 Go 单测覆盖。
func (h *Handler) WithVite(cfg ViteConfig) *Handler {
	h.vite = cfg
	return h
}

func (h *Handler) StudioPage(w http.ResponseWriter, r *http.Request) {
	if h.tmpl == nil {
		http.Error(w, "template not configured", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":   "Affiliate AI Studio",
		"DevMode": h.vite.DevMode,
	}

	if h.vite.DevMode {
		base := strings.TrimRight(h.vite.DevServerURL, "/")
		entry := h.vite.EntryModule
		if entry == "" {
			entry = "/src/main.tsx"
		}
		data["DevClientSrc"] = base + "/@vite/client"
		data["DevEntrySrc"] = base + entry
	} else if h.vite.Manifest != nil {
		entry, err := h.vite.Manifest.Resolve()
		if err != nil {
			log.Printf("studio page asset manifest error: %v", err)
			http.Error(w,
				"前端资源未就绪：请先在 web-studio 目录下运行 `npm install && npm run build`，或设置环境变量 AFFILIATE_STUDIO_DEV=1 走本地 Vite dev",
				http.StatusInternalServerError)
			return
		}
		data["JSFile"] = entry.File
		if len(entry.CSS) > 0 {
			data["CSSFiles"] = entry.CSS
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.ExecuteTemplate(w, "studio.html", data)
}

func (h *Handler) LearnPage(w http.ResponseWriter, r *http.Request) {
	if h.tmpl == nil {
		http.Error(w, "template not configured", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.ExecuteTemplate(w, "learn.html", map[string]any{
		"Title": "Learn · Affiliate AI Studio",
	})
}

func (h *Handler) RunAffiliateFlow(w http.ResponseWriter, r *http.Request) {
	var req RunAffiliateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "validation_error", "invalid JSON payload")
		return
	}
	if err := req.Validate(); err != nil {
		writeAPIError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if h.app == nil || h.app.Worker == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "worker_error", "worker is unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	sessionID, resp, err := h.app.RunAffiliateFlow(ctx, req)
	if err != nil {
		code := "worker_error"
		if resp.Error != nil && resp.Error.Code != "" {
			code = resp.Error.Code
		}
		if errors.Is(err, context.DeadlineExceeded) {
			code = "worker_error"
		}
		workerMsg := ""
		if resp.Error != nil {
			workerMsg = resp.Error.Message
		}
		log.Printf(
			"run_affiliate_flow error session=%s code=%s product_url=%q product_text_len=%d err=%v worker_msg=%q",
			sessionID, code, req.ProductURL, len(req.ProductText), err, workerMsg,
		)
		writeAPIError(w, http.StatusBadGateway, code, err.Error())
		return
	}
	view, err := viewmodel.FromWorkerResponse(sessionID, resp)
	if err != nil {
		log.Printf(
			"run_affiliate_flow decode error session=%s err=%v raw=%s",
			sessionID, err, string(resp.Data),
		)
		writeAPIError(w, http.StatusBadGateway, "worker_error", "invalid worker response")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	ready := h.app != nil && h.app.Worker != nil && h.app.Worker.Ready()
	statusCode := http.StatusServiceUnavailable
	status := "not_ready"
	if ready {
		statusCode = http.StatusOK
		status = "ok"
	}
	writeJSON(w, statusCode, map[string]any{"status": status})
}

func (r RunAffiliateRequest) Validate() error {
	if strings.TrimSpace(r.Platform) == "" {
		return errors.New("platform is required")
	}
	if strings.TrimSpace(r.Locale) == "" {
		return errors.New("locale is required")
	}
	if strings.TrimSpace(r.Style) == "" {
		return errors.New("style is required")
	}
	if strings.TrimSpace(r.ProductURL) == "" && strings.TrimSpace(r.ProductText) == "" {
		return errors.New("product_url or product_text is required")
	}
	if r.MinCommissionRate < 0 {
		return errors.New("min_commission_rate must be non-negative")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, statusCode int, code, message string) {
	writeJSON(w, statusCode, map[string]any{
		"status": "error",
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
