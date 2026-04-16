package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-learn-playground/affiliate-ai-studio/go/internal/app"
	"ai-learn-playground/affiliate-ai-studio/go/internal/worker"
)

func newTestTemplate(t *testing.T) *template.Template {
	t.Helper()
	return template.Must(template.New("studio").Parse(`<html><body>Affiliate AI Studio</body></html>`))
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

func TestStudioPageRenders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/studio", nil)
	rec := httptest.NewRecorder()

	handler := NewHandler(nil, newTestTemplate(t))
	handler.StudioPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Affiliate AI Studio") {
		t.Fatalf("expected studio page content, got %q", rec.Body.String())
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
