package httpserver

import "net/http"

func NewMux(handler *Handler, static http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/studio", http.StatusFound)
	})
	mux.HandleFunc("GET /studio", handler.StudioPage)
	mux.HandleFunc("POST /api/affiliate/run", handler.RunAffiliateFlow)
	mux.HandleFunc("GET /healthz", handler.Healthz)
	mux.HandleFunc("GET /readyz", handler.Readyz)
	if static != nil {
		mux.Handle("/static/", http.StripPrefix("/static/", static))
	}
	return mux
}
