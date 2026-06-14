package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

// stubTool 是一个最小 Tool 实现, 测试 /tools 路由用.
type stubTool struct {
	name string
	desc string
}

func (s stubTool) Schema() llm.ToolSchema {
	return tools.Schema(s.name, s.desc, map[string]any{"type": "object"})
}
func (s stubTool) Invoke(_ context.Context, args json.RawMessage) (string, error) {
	return string(args), nil
}

func newToolsServer(t *testing.T) *Server {
	t.Helper()
	reg := tools.NewRegistry()
	reg.Register(stubTool{name: "echo", desc: "echo back"})
	srv := newTestServer(t, &fakeStreamClient{})
	// newTestServer 返回的 server 没注入 tools, 这里重建一个带 registry 的.
	srv2, err := NewServer(srv.cfg, srv.llm, WithToolRegistry(reg))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv2
}

func TestToolsPage_Renders(t *testing.T) {
	srv := newToolsServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tools", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"Tools", "echo", "echo back", "/static/tools.js"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q\n%s", want, body)
		}
	}
}

func TestToolsInvoke_OK(t *testing.T) {
	srv := newToolsServer(t)
	body, _ := json.Marshal(map[string]any{
		"name": "echo",
		"args": map[string]any{"k": "v"},
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tools/invoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		OK     bool   `json:"ok"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok, got: %s", rr.Body.String())
	}
	if !strings.Contains(resp.Result, "v") {
		t.Fatalf("expected v in result: %q", resp.Result)
	}
}

func TestToolsInvoke_Unknown(t *testing.T) {
	srv := newToolsServer(t)
	body, _ := json.Marshal(map[string]any{"name": "missing", "args": map[string]any{}})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tools/invoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestToolsRecent_Buffer(t *testing.T) {
	srv := newToolsServer(t)
	// 调一次, recent 应有 1 条.
	body, _ := json.Marshal(map[string]any{"name": "echo", "args": map[string]any{"x": 1}})
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/tools/invoke", bytes.NewReader(body)))
	rr2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/tools/recent", nil))
	if rr2.Code != http.StatusOK {
		t.Fatalf("recent status: %d", rr2.Code)
	}
	if !strings.Contains(rr2.Body.String(), `"name":"echo"`) {
		t.Fatalf("recent missing echo: %s", rr2.Body.String())
	}
}

func TestToolsPage_DisabledWhenNoRegistry(t *testing.T) {
	// 默认 newTestServer 没注入 registry, /tools 应当走占位.
	srv := newTestServer(t, &fakeStreamClient{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tools", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 placeholder, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "M2 启用后开放") {
		t.Fatalf("expected placeholder hint, got: %s", rr.Body.String())
	}
}
