package web

import (
	"encoding/json"
	"net/http"

	"ai-learn-playground/agent-lab/internal/rag"
)

type knowledgePageData struct {
	Title   string
	Active  string
	Profile string
	Model   string
	Embed   string
}

func (s *Server) handleKnowledgePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := knowledgePageData{
		Title:   "Knowledge · agent-lab",
		Active:  "/knowledge",
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
		Embed:   s.cfg.ModelEmbed,
	}
	s.renderPage(w, "knowledge.html", data)
}

// handleKnowledgeAPI:
//   - GET  /api/knowledge          -> 列出所有 source 与块数.
//   - POST /api/knowledge/search   -> 检索 {query, k}, 返回 top-k 结果.
func (s *Server) handleKnowledgeAPI(w http.ResponseWriter, r *http.Request) {
	if s.retriever == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "retriever not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		sources, err := s.retriever.Sources(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"sources": sources,
			"count":   s.retriever.Count(),
			"dim":     s.retriever.Dim(),
		})
	case http.MethodPost:
		s.handleKnowledgeSearch(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		K     int    `json:"k"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query is required"})
		return
	}
	if req.K <= 0 {
		req.K = 5
	}
	results, err := s.retriever.Retrieve(r.Context(), req.Query, req.K)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	rendered := rag.Render(results)
	writeJSON(w, http.StatusOK, map[string]any{
		"query":   req.Query,
		"count":   len(results),
		"results": results,
		"context": rendered,
	})
}

// readJSON 是 writeJSON 的逆操作, 从 r.Body 解码 JSON.
func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
