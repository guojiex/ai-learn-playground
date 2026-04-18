package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-learn-playground/affiliate-ai-studio/go/internal/app"
	"ai-learn-playground/affiliate-ai-studio/go/internal/assets"
	"ai-learn-playground/affiliate-ai-studio/go/internal/worker"
)

const studioTemplate = `<html><head><title>{{ .Title }}</title>
{{ if .DevMode }}<script src="{{ .DevEntrySrc }}"></script>
{{ else }}
  {{ range .CSSFiles }}<link rel="stylesheet" href="/static/dist{{ . }}" />{{ end }}
  <script type="module" src="/static/dist{{ .JSFile }}"></script>
{{ end }}
</head><body><div id="root"></div></body></html>`

func newTestTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl := template.Must(template.New("studio.html").Parse(studioTemplate))
	template.Must(tmpl.New("learn.html").Parse(`<html><body>Learn AI Together</body></html>`))
	return tmpl
}

func writeManifest(t *testing.T, data string) *assets.Manifest {
	t.Helper()
	dir := t.TempDir()
	viteDir := filepath.Join(dir, ".vite")
	if err := os.MkdirAll(viteDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(viteDir, "manifest.json"), []byte(data), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return assets.NewManifest(dir, "src/main.tsx")
}

type stubWorker struct {
	response worker.Response
	err      error
	ready    bool
}

func (s stubWorker) Call(ctx context.Context, req worker.Request) (worker.Response, error) {
	return s.response, s.err
}

func (s stubWorker) Ready() bool {
	return s.ready
}

func (s stubWorker) Close() error {
	return nil
}

func TestStudioPageRendersWithManifest(t *testing.T) {
	manifest := writeManifest(t, `{
		"src/main.tsx": {
			"file": "assets/main-abc.js",
			"css": ["assets/main-xyz.css"]
		}
	}`)

	req := httptest.NewRequest(http.MethodGet, "/studio", nil)
	rec := httptest.NewRecorder()

	handler := NewHandler(nil, newTestTemplate(t)).
		WithVite(ViteConfig{Manifest: manifest})
	handler.StudioPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Affiliate AI Studio") {
		t.Fatalf("expected title, got %q", body)
	}
	if !strings.Contains(body, "/static/dist/assets/main-abc.js") {
		t.Fatalf("expected hashed JS, got %q", body)
	}
	if !strings.Contains(body, "/static/dist/assets/main-xyz.css") {
		t.Fatalf("expected hashed CSS, got %q", body)
	}
}

func TestStudioPageDevModeUsesViteDevServer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/studio", nil)
	rec := httptest.NewRecorder()

	handler := NewHandler(nil, newTestTemplate(t)).
		WithVite(ViteConfig{
			DevMode:      true,
			DevServerURL: "http://localhost:5173",
			EntryModule:  "/src/main.tsx",
		})
	handler.StudioPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "http://localhost:5173/src/main.tsx") {
		t.Fatalf("expected vite dev server src, got %q", body)
	}
}

func TestStudioPageMissingManifestReturns500(t *testing.T) {
	manifest := assets.NewManifest(t.TempDir(), "src/main.tsx")

	req := httptest.NewRequest(http.MethodGet, "/studio", nil)
	rec := httptest.NewRecorder()

	handler := NewHandler(nil, newTestTemplate(t)).
		WithVite(ViteConfig{Manifest: manifest})
	handler.StudioPage(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "npm run build") {
		t.Fatalf("expected hint in body, got %q", rec.Body.String())
	}
}

func TestLearnPageRenders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/learn", nil)
	rec := httptest.NewRecorder()

	handler := NewHandler(nil, newTestTemplate(t))
	handler.LearnPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Learn AI Together") {
		t.Fatalf("expected learn page content, got %q", rec.Body.String())
	}
}

func TestRunAffiliateRejectsEmptyPayload(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/affiliate/run", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := NewHandler(nil, newTestTemplate(t))
	handler.RunAffiliateFlow(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "validation_error") || !strings.Contains(rec.Body.String(), "platform") {
		t.Fatalf("expected validation error body, got %q", rec.Body.String())
	}
}

func TestRunAffiliateReturnsWorkerErrorCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/affiliate/run", strings.NewReader(`{"product_url":"https://shop.example/power-bank-10000","platform":"TikTok","locale":"id-ID","style":"casual","min_commission_rate":0.1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := NewHandler(app.New("Affiliate AI Studio", stubWorker{err: context.DeadlineExceeded, ready: true}), newTestTemplate(t))
	handler.RunAffiliateFlow(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "worker_error") {
		t.Fatalf("expected worker_error body, got %q", rec.Body.String())
	}
}

func TestRunAffiliatePreservesToolErrorCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/affiliate/run", strings.NewReader(`{"product_url":"https://shop.example/power-bank-10000","platform":"TikTok","locale":"id-ID","style":"casual","min_commission_rate":0.1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := NewHandler(app.New("Affiliate AI Studio", stubWorker{
		err:   errors.New("no matching product fixture found"),
		ready: true,
		response: worker.Response{
			Status: "error",
			Error:  &worker.ErrorPayload{Code: "tool_error", Message: "no matching product fixture found"},
		},
	}), newTestTemplate(t))
	handler.RunAffiliateFlow(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tool_error") {
		t.Fatalf("expected tool_error body, got %q", rec.Body.String())
	}
}

func TestReadyzReflectsWorkerState(t *testing.T) {
	readyHandler := NewHandler(app.New("Affiliate AI Studio", stubWorker{ready: true}), newTestTemplate(t))
	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyRec := httptest.NewRecorder()
	readyHandler.Readyz(readyRec, readyReq)
	if readyRec.Code != http.StatusOK {
		t.Fatalf("expected ready status 200, got %d", readyRec.Code)
	}

	notReadyHandler := NewHandler(app.New("Affiliate AI Studio", stubWorker{ready: false}), newTestTemplate(t))
	notReadyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	notReadyRec := httptest.NewRecorder()
	notReadyHandler.Readyz(notReadyRec, notReadyReq)
	if notReadyRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected not-ready status 503, got %d", notReadyRec.Code)
	}
}

func TestRunAffiliateReturnsStructuredJSON(t *testing.T) {
	data, err := json.Marshal(map[string]any{
		"result": map[string]any{"decision": "accepted"},
	})
	if err != nil {
		t.Fatalf("marshal stub response: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/affiliate/run", strings.NewReader(`{"product_url":"https://shop.example/power-bank-10000","platform":"TikTok","locale":"id-ID","style":"casual","min_commission_rate":0.1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := NewHandler(app.New("Affiliate AI Studio", stubWorker{
		ready:    true,
		response: worker.Response{Status: "ok", Data: data},
	}), newTestTemplate(t))
	handler.RunAffiliateFlow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "\"decision\":\"accepted\"") {
		t.Fatalf("expected structured result body, got %q", rec.Body.String())
	}
}
