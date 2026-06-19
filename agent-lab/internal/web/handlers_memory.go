package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"ai-learn-playground/agent-lab/internal/store"
)

type memoryPageData struct {
	Title   string
	Active  string
	Profile string
	Model   string
}

func (s *Server) handleMemoryPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := memoryPageData{
		Title:   "Memory · agent-lab",
		Active:  "/memory",
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
	}
	s.renderPage(w, "memory.html", data)
}

// memoryNamespaceView 是 /api/memory 返回的单个 namespace 视图.
type memoryNamespaceView struct {
	Namespace string          `json:"namespace"`
	Entries   []store.KVEntry `json:"entries"`
}

// handleMemoryAPI:
//   - GET    /api/memory          -> 列出所有 namespace 及其键值 (折叠树用).
//   - DELETE /api/memory          -> 遗忘一个键 {namespace, key} (进阶练习: 被遗忘权).
func (s *Server) handleMemoryAPI(w http.ResponseWriter, r *http.Request) {
	if s.kv == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "memory not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleMemoryList(w, r)
	case http.MethodDelete:
		s.handleMemoryForget(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMemoryList(w http.ResponseWriter, r *http.Request) {
	namespaces, err := s.kv.Namespaces(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	views := make([]memoryNamespaceView, 0, len(namespaces))
	for _, ns := range namespaces {
		entries, err := s.kv.List(r.Context(), ns)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		views = append(views, memoryNamespaceView{Namespace: ns, Entries: entries})
	}
	writeJSON(w, http.StatusOK, map[string]any{"namespaces": views})
}

func (s *Server) handleMemoryForget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Namespace string `json:"namespace"`
		Key       string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json: " + err.Error()})
		return
	}
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Key = strings.TrimSpace(req.Key)
	if req.Namespace == "" || req.Key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "namespace and key are required"})
		return
	}
	if err := s.kv.Delete(r.Context(), req.Namespace, req.Key); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "namespace": req.Namespace, "key": req.Key})
}
