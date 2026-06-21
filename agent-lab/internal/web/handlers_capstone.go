package web

import (
	"context"
	"net/http"
	"time"

	"ai-learn-playground/agent-lab/internal/capstone"
	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

type capstonePageData struct {
	Title   string
	Active  string
	Profile string
	Model   string
}

func (s *Server) handleCapstonePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := capstonePageData{
		Title:   "Capstone · agent-lab",
		Active:  "/capstone",
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
	}
	s.renderPage(w, "capstone.html", data)
}

// handleCapstoneRun: POST /api/capstone/run → SSE 流式推送 pipeline 进度
func (s *Server) handleCapstoneRun(w http.ResponseWriter, r *http.Request) {
	if s.llm == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "llm not configured"})
		return
	}
	var req struct {
		Seller    string   `json:"seller"`
		SKUID     string   `json:"sku_id"`
		Platforms []string `json:"platforms"`
		Style     string   `json:"style"`
		MaxRounds int      `json:"max_rounds"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.SKUID == "" {
		req.SKUID = "sku_001"
	}
	if len(req.Platforms) == 0 {
		req.Platforms = []string{"shopee", "xhs"}
	}
	if req.Style == "" {
		req.Style = "girlfriend"
	}
	if req.MaxRounds <= 0 {
		req.MaxRounds = 3
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")

	writeSSE(w, flusher, "start", map[string]any{
		"seller": req.Seller, "sku_id": req.SKUID,
		"platforms": req.Platforms, "style": req.Style,
	})

	reg := tools.NewRegistry()
	if s.tools != nil {
		for _, name := range s.tools.Names() {
			if t, ok := s.tools.Get(name); ok {
				reg.Register(t)
			}
		}
	}

	pipeline := capstone.NewPipeline(s.llm, reg, s.cfg.ModelChat)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	result, err := pipeline.Run(ctx, capstone.PipelineInput{
		Seller:    req.Seller,
		SKUID:     req.SKUID,
		Platforms: req.Platforms,
		Style:     req.Style,
		MaxRounds: req.MaxRounds,
	})

	if err != nil {
		writeSSE(w, flusher, "error", map[string]any{"error": err.Error()})
		return
	}

	writeSSE(w, flusher, "done", map[string]any{
		"result": result,
		"report": capstone.RenderReport(result),
	})
}

var _ = llm.RoleSystem
