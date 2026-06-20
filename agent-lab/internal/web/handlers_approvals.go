package web

import (
	"net/http"
	"time"

	"ai-learn-playground/agent-lab/internal/hitl"
)

type approvalsPageData struct {
	Title   string
	Active  string
	Profile string
	Model   string
}

func (s *Server) handleApprovalsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := approvalsPageData{
		Title:   "Approvals · agent-lab",
		Active:  "/approvals",
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
	}
	s.renderPage(w, "approvals.html", data)
}

// handleApprovalsAPI:
//   - GET  /api/approvals          -> {pending: [...], count: N}
//   - POST /api/approvals/{id}/approve  -> approve
//   - POST /api/approvals/{id}/reject   -> reject
//   - POST /api/approvals/{id}/edit     -> edit args
func (s *Server) handleApprovalsAPI(w http.ResponseWriter, r *http.Request) {
	if s.approvals == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "approvals not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		pending, err := s.approvals.ListPending(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		count, _ := s.approvals.CountPending(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{
			"pending": pending,
			"count":   count,
		})
	case http.MethodPost:
		s.handleApprovalAction(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleApprovalAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Action string `json:"action"` // "approve" | "reject" | "edit"
		Note   string `json:"note"`
		Args   string `json:"args"` // only for "edit"
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	reviewer := "web"
	var (
		a   *hitl.Approval
		err error
	)
	switch req.Action {
	case "approve":
		a, err = s.approvals.Approve(r.Context(), req.ID, reviewer, req.Note)
	case "reject":
		a, err = s.approvals.Reject(r.Context(), req.ID, reviewer, req.Note)
	case "edit":
		a, err = s.approvals.Edit(r.Context(), req.ID, reviewer, req.Note, req.Args)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown action: " + req.Action})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"approval": a,
		"at":       time.Now().Format(time.RFC3339),
	})
}
