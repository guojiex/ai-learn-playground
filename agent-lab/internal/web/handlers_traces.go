package web

import (
	"net/http"

	"ai-learn-playground/agent-lab/internal/trace"
)

type tracesPageData struct {
	Title   string
	Active  string
	Profile string
	Model   string
}

func (s *Server) handleTracesPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := tracesPageData{
		Title:   "Traces · agent-lab",
		Active:  "/traces",
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
	}
	s.renderPage(w, "traces.html", data)
}

// handleTracesAPI:
//   - GET /api/traces          -> {traces: [...]} 最近 N 条
//   - GET /api/traces?id=<id>  -> 单条 trace + spans
func (s *Server) handleTracesAPI(w http.ResponseWriter, r *http.Request) {
	if s.recorder == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "trace not configured"})
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if id := r.URL.Query().Get("id"); id != "" {
		t, err := s.recorder.GetTrace(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, t)
		return
	}
	limit := 50
	traces, err := s.recorder.ListTraces(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"traces": traces,
		"count":  len(traces),
	})
}

var _ = trace.SpanLLM
