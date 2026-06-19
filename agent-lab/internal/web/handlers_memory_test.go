package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/store"
)

func newServerWithMemory(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	kv := memory.NewKV(st)
	return newTestServer(t, &fakeStreamClient{}, WithStore(st), WithMemory(kv))
}

func TestMemoryPage_Renders(t *testing.T) {
	srv := newServerWithMemory(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/memory", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"Memory", "/static/memory.js", "长期记忆"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestMemoryPage_PlaceholderWhenNotConfigured(t *testing.T) {
	srv := newTestServer(t, &fakeStreamClient{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/memory", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "M4") {
		t.Fatalf("expected M4 placeholder, got: %s", rr.Body.String())
	}
}

func TestMemoryAPI_ListAndForget(t *testing.T) {
	srv := newServerWithMemory(t)
	ctx := context.Background()
	if err := srv.kv.Put(ctx, "seller:A001", "tone", `{"style":"girlfriend"}`); err != nil {
		t.Fatal(err)
	}
	if err := srv.kv.Put(ctx, "seller:A001", "keywords", `["現貨"]`); err != nil {
		t.Fatal(err)
	}

	// GET /api/memory -> 1 namespace, 2 entries
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/memory", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status %d body %s", rr.Code, rr.Body.String())
	}
	var data map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	nss, ok := data["namespaces"].([]any)
	if !ok || len(nss) != 1 {
		t.Fatalf("namespaces=%v", data["namespaces"])
	}
	ns0 := nss[0].(map[string]any)
	if ns0["namespace"] != "seller:A001" {
		t.Fatalf("namespace=%v", ns0["namespace"])
	}
	entries := ns0["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("entries=%d", len(entries))
	}

	// DELETE /api/memory -> forget one key
	body, _ := json.Marshal(map[string]string{"namespace": "seller:A001", "key": "tone"})
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodDelete, "/api/memory", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	srv.Routes().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("delete status %d body %s", rr2.Code, rr2.Body.String())
	}
	if _, found, _ := srv.kv.Get(ctx, "seller:A001", "tone"); found {
		t.Fatal("tone still found after forget")
	}
	// 另一个键仍在
	if _, found, _ := srv.kv.Get(ctx, "seller:A001", "keywords"); !found {
		t.Fatal("keywords should still exist")
	}
}
