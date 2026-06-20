package web

import (
	"context"
	"encoding/json"
	"net/http"

	"ai-learn-playground/agent-lab/internal/agent"
)

type multiPageData struct {
	Title   string
	Active  string
	Profile string
	Model   string
}

func (s *Server) handleMultiPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := multiPageData{
		Title:   "Multi-Agent · agent-lab",
		Active:  "/multi",
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
	}
	s.renderPage(w, "multi.html", data)
}

// handleMultiRun: POST /api/multi/run {goal, max_rounds} → SSE 流式推送协作进度
func (s *Server) handleMultiRun(w http.ResponseWriter, r *http.Request) {
	if s.multiFactory == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "multi-agent not configured"})
		return
	}
	var req struct {
		Goal      string `json:"goal"`
		MaxRounds int    `json:"max_rounds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Goal == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "goal is required"})
		return
	}
	if req.MaxRounds <= 0 {
		req.MaxRounds = 4
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

	runID := agent.NextRunID()
	bus := agent.NewMessageBus(runID, s.store)
	multi := s.multiFactory(bus)
	multi.SetMaxRounds(req.MaxRounds)

	events := make(chan agent.MultiEvent, 32)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		result, _ := multi.Run(ctx, req.Goal, events)
		_ = result
		close(events)
	}()

	for ev := range events {
		writeSSE(w, flusher, ev.Type, ev)
	}
}
