package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"ai-learn-playground/agent-lab/internal/agent"
)

type planPageData struct {
	Title   string
	Active  string
	Profile string
	Model   string
}

func (s *Server) handlePlanPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := planPageData{
		Title:   "Plan · agent-lab",
		Active:  "/plan",
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
	}
	s.renderPage(w, "plan.html", data)
}

// handlePlanGenerate: POST /api/plan/generate {goal} → {plan}
func (s *Server) handlePlanGenerate(w http.ResponseWriter, r *http.Request) {
	if s.planner == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "planner not configured"})
		return
	}
	var req struct {
		Goal string `json:"goal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Goal == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "goal is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.RequestTimeout)
	defer cancel()
	plan, err := s.planner.Plan(ctx, req.Goal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plan":   plan,
		"levels": plan.TopoLevels(),
	})
}

// handlePlanExecute: POST /api/plan/execute {plan} → SSE 流式推送执行进度
func (s *Server) handlePlanExecute(w http.ResponseWriter, r *http.Request) {
	if s.executor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "executor not configured"})
		return
	}
	var req struct {
		Plan *agent.Plan `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Plan == nil || len(req.Plan.Tasks) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "plan is required"})
		return
	}
	if err := req.Plan.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("invalid plan: %s", err)})
		return
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

	events := make(chan agent.ExecEvent, 32)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		_, _ = s.executor.Execute(ctx, req.Plan, events)
		close(events)
	}()

	for ev := range events {
		writeSSE(w, flusher, ev.Type, ev)
	}
}
