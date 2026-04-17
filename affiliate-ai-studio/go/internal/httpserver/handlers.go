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
	"ai-learn-playground/affiliate-ai-studio/go/internal/viewmodel"
)

type Handler struct {
	app  *app.App
	tmpl *template.Template
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

func (h *Handler) StudioPage(w http.ResponseWriter, r *http.Request) {
	if h.tmpl == nil {
		http.Error(w, "template not configured", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.ExecuteTemplate(w, "studio.html", map[string]any{
		"Title": "Affiliate AI Studio",
	})
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

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
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
