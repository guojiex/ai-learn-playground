package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ai-learn-playground/agent-lab/internal/memory"
	"ai-learn-playground/agent-lab/internal/rag"
	"ai-learn-playground/agent-lab/internal/store"
)

// fakeWebEmbedder 满足 llm.Embedder, 供 web 测试用.
type fakeWebEmbedder struct{ dim int }

func (e *fakeWebEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, e.dim)
		for _, r := range t {
			v[int(r)%e.dim] += 1
		}
		out[i] = v
	}
	return out, nil
}

func (e *fakeWebEmbedder) Dim() int { return e.dim }

func newServerWithRetriever(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	vs, err := memory.NewVectorStore(st)
	if err != nil {
		t.Fatalf("vs: %v", err)
	}
	emb := &fakeWebEmbedder{dim: 64}
	// 写入测试文档.
	ctx := context.Background()
	docs := []string{"蝦皮標題120字元", "momo標題60字元"}
	for i, d := range docs {
		vec, _ := emb.Embed(ctx, []string{d})
		_ = vs.Add(ctx, "doc"+string(rune('0'+i)), "test", i, d, vec[0], "{}")
	}
	r := rag.NewRetriever(emb, vs)
	return newTestServer(t, &fakeStreamClient{}, WithRetriever(r))
}

func TestKnowledgePage_Renders(t *testing.T) {
	srv := newServerWithRetriever(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/knowledge", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"Knowledge", "/static/knowledge.js", "RAG"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestKnowledgeAPI_ListAndSearch(t *testing.T) {
	srv := newServerWithRetriever(t)

	// GET /api/knowledge → 统计.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/knowledge", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status %d", rr.Code)
	}
	var data map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &data)
	if data["count"].(float64) != 2 {
		t.Fatalf("count=%v, want 2", data["count"])
	}

	// POST /api/knowledge/search.
	body := `{"query":"蝦皮標題","k":2}`
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/knowledge", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	srv.Routes().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("search status %d body %s", rr2.Code, rr2.Body.String())
	}
	var sdata map[string]any
	_ = json.Unmarshal(rr2.Body.Bytes(), &sdata)
	if sdata["count"].(float64) == 0 {
		t.Fatal("expected non-zero search results")
	}
	results := sdata["results"].([]any)
	if len(results) == 0 {
		t.Fatal("expected results array")
	}
}

func TestKnowledgePage_PlaceholderWhenNotConfigured(t *testing.T) {
	srv := newTestServer(t, &fakeStreamClient{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/knowledge", nil)
	srv.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "M5") {
		t.Fatalf("expected M5 placeholder, got: %s", rr.Body.String())
	}
}
