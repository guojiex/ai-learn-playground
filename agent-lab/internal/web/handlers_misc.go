package web

import (
	"net/http"
)

type placeholderData struct {
	Title     string
	Active    string
	Label     string
	Milestone string
	Note      string
}

func (s *Server) renderPlaceholder(w http.ResponseWriter, p Placeholder) {
	if p.Path == "/settings" {
		s.renderSettings(w, p)
		return
	}
	data := placeholderData{
		Title:     p.Label + " · agent-lab",
		Active:    p.Path,
		Label:     p.Label,
		Milestone: p.Milestone,
		Note:      p.Note,
	}
	s.renderPage(w, "placeholder.html", data)
}

type settingsData struct {
	Title          string
	Active         string
	Profile        string
	BaseURL        string
	APIKeyMasked   string
	ModelChat      string
	RequestTimeout string
	MaxRetries     int
}

func (s *Server) renderSettings(w http.ResponseWriter, p Placeholder) {
	data := settingsData{
		Title:          "Settings · agent-lab",
		Active:         p.Path,
		Profile:        s.cfg.Profile,
		BaseURL:        s.cfg.BaseURL,
		APIKeyMasked:   maskKey(s.cfg.APIKey),
		ModelChat:      s.cfg.ModelChat,
		RequestTimeout: s.cfg.RequestTimeout.String(),
		MaxRetries:     s.cfg.MaxRetries,
	}
	s.renderPage(w, "settings.html", data)
}

func (s *Server) renderNotFound(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	data := placeholderData{
		Title:     "Not Found · agent-lab",
		Active:    "",
		Label:     "Not Found",
		Milestone: "",
		Note:      "请检查地址栏路径.",
	}
	s.renderPage(w, "placeholder.html", data)
}

func maskKey(k string) string {
	if len(k) <= 4 {
		return "***"
	}
	return k[:2] + "***" + k[len(k)-2:]
}
