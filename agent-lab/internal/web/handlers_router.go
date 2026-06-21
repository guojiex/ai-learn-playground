package web

import (
	"net/http"
)

type routerPageData struct {
	Title   string
	Active  string
	Profile string
	Model   string
}

func (s *Server) handleRouterPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := routerPageData{
		Title:   "Router · agent-lab",
		Active:  "/router",
		Profile: s.cfg.Profile,
		Model:   s.cfg.ModelChat,
	}
	s.renderPage(w, "router.html", data)
}

// handleRouterAPI: GET /api/router → {registry, policy, recent}
func (s *Server) handleRouterAPI(w http.ResponseWriter, r *http.Request) {
	if s.router == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "router not configured"})
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	reg := s.router.Registry()
	policy := s.router.Policy()
	recent := s.router.RecentRoutes(50)
	writeJSON(w, http.StatusOK, map[string]any{
		"registry": reg,
		"policy":   policy,
		"recent":   recent,
		"count":    len(recent),
	})
}
