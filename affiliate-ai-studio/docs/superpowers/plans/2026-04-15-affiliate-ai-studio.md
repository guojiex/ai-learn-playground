# Affiliate AI Studio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a local-first web demo that uses Go as the web and API shell, Python as the LangChain and LangGraph runtime, and local fixture data plus a local model adapter to demonstrate prompt, RAG, tool, and graph workflows.

**Architecture:** Create a new `affiliate-ai-studio/` project with a Go `ServeMux` server, a long-lived Python JSONL worker, local KB assets, and a single studio page. The main business flow rejects low-commission products early, otherwise retrieves policy context and generates structured affiliate copy with trace output.

**Tech Stack:** Go 1.22 `net/http`, `html/template`, Python 3.11, `langchain`, `langgraph`, `pydantic`, `chromadb`, local fixture-backed tools, and a local model adapter with deterministic fallback.

---

### Task 1: Scaffold the project skeleton

**Files:**
- Create: `affiliate-ai-studio/README.md`
- Create: `affiliate-ai-studio/Makefile`
- Create: `affiliate-ai-studio/.gitignore`
- Create: `affiliate-ai-studio/go/go.mod`
- Create: `affiliate-ai-studio/go/cmd/studio/main.go`
- Create: `affiliate-ai-studio/python/requirements.txt`
- Create: `affiliate-ai-studio/python/main.py`
- Create: `affiliate-ai-studio/assets/products/sample_products.json`
- Create: `affiliate-ai-studio/assets/products/commission_rates.json`
- Create: `affiliate-ai-studio/assets/kb/policies/affiliate_policy.md`
- Create: `affiliate-ai-studio/assets/kb/campaigns/ramadan_campaign.md`

- [ ] **Step 1: Write the failing Go smoke test**

```go
package main

import "testing"

func TestProjectScaffoldExists(t *testing.T) {
	t.Fatal("project scaffold not created")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd affiliate-ai-studio/go && go test ./...`
Expected: FAIL because the smoke test intentionally fails and the server scaffold does not exist yet.

- [ ] **Step 3: Write minimal scaffold**

```go
package main

import "log"

func main() {
	log.Println("affiliate-ai-studio bootstrap")
}
```

```python
def main() -> None:
    print('{"status":"bootstrap"}', flush=True)


if __name__ == "__main__":
    main()
```

- [ ] **Step 4: Run test to verify it passes after updating the smoke test**

Run: `cd affiliate-ai-studio/go && go test ./...`
Expected: PASS after replacing the intentional failure with a real scaffold assertion.

- [ ] **Step 5: Commit**

```bash
git add affiliate-ai-studio
git commit -m "feat: scaffold affiliate ai studio"
```

### Task 2: Build the Go HTTP shell and studio page

**Files:**
- Create: `affiliate-ai-studio/go/internal/app/app.go`
- Create: `affiliate-ai-studio/go/internal/httpserver/routes.go`
- Create: `affiliate-ai-studio/go/internal/httpserver/handlers.go`
- Create: `affiliate-ai-studio/go/internal/httpserver/handlers_test.go`
- Create: `affiliate-ai-studio/go/web/templates/studio.html`
- Create: `affiliate-ai-studio/go/web/static/app.js`
- Create: `affiliate-ai-studio/go/web/static/app.css`

- [ ] **Step 1: Write the failing handler tests**

```go
func TestStudioPageRenders(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/studio", nil)
    rec := httptest.NewRecorder()
    handler := NewHandler(nil)
    handler.StudioPage(rec, req)
    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rec.Code)
    }
}

func TestRunAffiliateRejectsEmptyPayload(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/api/affiliate/run", strings.NewReader(`{}`))
    rec := httptest.NewRecorder()
    handler := NewHandler(nil)
    handler.RunAffiliateFlow(rec, req)
    if rec.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rec.Code)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd affiliate-ai-studio/go && go test ./internal/httpserver -v`
Expected: FAIL because handlers and routes do not exist yet.

- [ ] **Step 3: Write minimal HTTP implementation**

```go
mux.HandleFunc("GET /studio", h.StudioPage)
mux.HandleFunc("POST /api/affiliate/run", h.RunAffiliateFlow)
mux.HandleFunc("GET /healthz", h.Healthz)
mux.HandleFunc("GET /readyz", h.Readyz)
```

```go
func (h *Handler) RunAffiliateFlow(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusBadRequest, map[string]any{
        "status": "error",
        "error":  "missing required affiliate input",
    })
}
```

- [ ] **Step 4: Make the handlers pass with HTML and JSON responses**

Run: `cd affiliate-ai-studio/go && go test ./internal/httpserver -v`
Expected: PASS for page rendering and validation tests.

- [ ] **Step 5: Commit**

```bash
git add affiliate-ai-studio/go
git commit -m "feat: add studio web shell"
```

### Task 3: Build the Python worker bridge and fixture-backed tools

**Files:**
- Create: `affiliate-ai-studio/go/internal/worker/client.go`
- Create: `affiliate-ai-studio/go/internal/worker/client_test.go`
- Create: `affiliate-ai-studio/python/bridge/protocol.py`
- Create: `affiliate-ai-studio/python/bridge/runtime.py`
- Create: `affiliate-ai-studio/python/tools/product_tools.py`
- Create: `affiliate-ai-studio/python/tools/commission_tools.py`
- Create: `affiliate-ai-studio/python/tests/test_tools.py`

- [ ] **Step 1: Write the failing Go and Python tests**

```go
func TestClientRoundTrip(t *testing.T) {
    stub := bytes.NewBufferString("{\"status\":\"ok\"}\n")
    client := NewTestClient(stub, io.Discard)
    _, err := client.Call(context.Background(), Request{Action: "ping"})
    if err != nil {
        t.Fatalf("expected round trip to succeed: %v", err)
    }
}
```

```python
def test_lookup_product_by_url_returns_fixture():
    tool = ProductLookupTool(fixtures=[{"id": "p1", "url": "https://shop/p1", "title": "Power Bank"}])
    product = tool.lookup("https://shop/p1", "")
    assert product["id"] == "p1"


def test_lookup_commission_returns_rate():
    tool = CommissionLookupTool(rates={"p1": 0.18})
    assert tool.lookup("p1") == 0.18
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd affiliate-ai-studio/go && go test ./internal/worker -v`
Expected: FAIL because the client does not exist.

Run: `cd affiliate-ai-studio/python && python -m pytest tests/test_tools.py -q`
Expected: FAIL because the tools do not exist.

- [ ] **Step 3: Write minimal bridge and tools**

```go
type Request struct {
    Action    string          `json:"action"`
    SessionID string          `json:"session_id"`
    Payload   json.RawMessage `json:"payload"`
}
```

```python
def load_products(path: Path) -> list[dict]:
    return json.loads(path.read_text())

def lookup_commission(product_id: str, rates: dict[str, float]) -> float:
    return rates.get(product_id, 0.0)
```

- [ ] **Step 4: Re-run tests until they pass**

Run: `cd affiliate-ai-studio/go && go test ./internal/worker -v`
Expected: PASS.

Run: `cd affiliate-ai-studio/python && python -m pytest tests/test_tools.py -q`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add affiliate-ai-studio/go affiliate-ai-studio/python affiliate-ai-studio/assets
git commit -m "feat: add worker bridge and fixture tools"
```

### Task 4: Implement prompt lab, KB indexing, and retrieval helpers

**Files:**
- Create: `affiliate-ai-studio/python/schemas/models.py`
- Create: `affiliate-ai-studio/python/llm/local_model.py`
- Create: `affiliate-ai-studio/python/labs/prompt_lab.py`
- Create: `affiliate-ai-studio/python/rag/indexer.py`
- Create: `affiliate-ai-studio/python/rag/retriever.py`
- Create: `affiliate-ai-studio/python/tests/test_prompt_lab.py`
- Create: `affiliate-ai-studio/python/tests/test_retriever.py`

- [ ] **Step 1: Write the failing Python tests**

```python
def test_prompt_lab_returns_structured_copy():
    copy = generate_affiliate_copy({"title": "Power Bank"}, "TikTok", "id-ID", "casual")
    assert copy.title
    assert len(copy.selling_points) == 3


def test_retriever_returns_policy_hits():
    retriever = KeywordRetriever([{"id": "doc1", "text": "Avoid exaggerated claims in affiliate copy."}])
    hits = retriever.search("affiliate claims")
    assert hits[0]["id"] == "doc1"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd affiliate-ai-studio/python && python -m pytest tests/test_prompt_lab.py tests/test_retriever.py -q`
Expected: FAIL because schemas, model adapter, and retriever are missing.

- [ ] **Step 3: Write minimal implementation**

```python
class AffiliateCopy(BaseModel):
    title: str
    selling_points: list[str]
    localized_hook: str
    risk_notes: list[str]
```

```python
def generate_affiliate_copy(product_info: dict, platform: str, locale: str, style: str) -> AffiliateCopy:
    return AffiliateCopy(
        title=f"{platform} promotion for {product_info['title']}",
        selling_points=["Point 1", "Point 2", "Point 3"],
        localized_hook=f"Localized for {locale}",
        risk_notes=["Avoid exaggerated claims"],
    )
```

- [ ] **Step 4: Re-run tests**

Run: `cd affiliate-ai-studio/python && python -m pytest tests/test_prompt_lab.py tests/test_retriever.py -q`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add affiliate-ai-studio/python
git commit -m "feat: add prompt and rag labs"
```

### Task 5: Implement the LangGraph affiliate flow

**Files:**
- Create: `affiliate-ai-studio/python/graph/affiliate_graph.py`
- Create: `affiliate-ai-studio/python/tests/test_graph.py`
- Modify: `affiliate-ai-studio/python/main.py`

- [ ] **Step 1: Write the failing graph tests**

```python
def test_graph_rejects_low_commission_before_generation():
    result = run_affiliate_graph(make_runtime(commission=0.05), {
        "product_url": "https://shop/p1",
        "platform": "TikTok",
        "locale": "id-ID",
        "style": "casual",
        "min_commission_rate": 0.10,
    })
    assert result["decision"] == "rejected"


def test_graph_accepts_and_returns_copy_when_commission_meets_threshold():
    result = run_affiliate_graph(make_runtime(commission=0.18), {
        "product_url": "https://shop/p1",
        "platform": "TikTok",
        "locale": "id-ID",
        "style": "casual",
        "min_commission_rate": 0.10,
    })
    assert result["decision"] == "accepted"
    assert result["copy"]["title"]
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd affiliate-ai-studio/python && python -m pytest tests/test_graph.py -q`
Expected: FAIL because the graph and worker dispatch are missing.

- [ ] **Step 3: Write minimal graph implementation**

```python
def run_affiliate_graph(runtime: Runtime, payload: dict) -> dict:
    state = normalize_state(payload)
    product = runtime.products.lookup(state["product_url"], state["product_text"])
    commission = runtime.commissions.lookup(product["id"])
    if commission < state["min_commission_rate"]:
        return {"decision": "rejected", "commission_rate": commission, "trace": ["reject"]}
    hits = runtime.retriever.search(product["title"])
    copy = runtime.prompt_lab.generate(product, state["platform"], state["locale"], state["style"])
    return {"decision": "accepted", "commission_rate": commission, "retrieval": hits, "copy": copy.model_dump(), "trace": ["retrieve", "generate"]}
```

- [ ] **Step 4: Re-run tests**

Run: `cd affiliate-ai-studio/python && python -m pytest tests/test_graph.py -q`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add affiliate-ai-studio/python
git commit -m "feat: add affiliate graph runtime"
```

### Task 6: Integrate the Go shell with the Python worker

**Files:**
- Modify: `affiliate-ai-studio/go/internal/httpserver/handlers.go`
- Modify: `affiliate-ai-studio/go/internal/app/app.go`
- Create: `affiliate-ai-studio/go/internal/app/app_test.go`
- Modify: `affiliate-ai-studio/go/cmd/studio/main.go`

- [ ] **Step 1: Write the failing integration tests**

```go
func TestRunAffiliateFlowReturnsStructuredJSON(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/api/affiliate/run", strings.NewReader(`{"product_url":"https://shop/p1","platform":"TikTok","locale":"id-ID","style":"casual","min_commission_rate":0.1}`))
    rec := httptest.NewRecorder()
    handler := NewHandler(newStubApp())
    handler.RunAffiliateFlow(rec, req)
    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rec.Code)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd affiliate-ai-studio/go && go test ./internal/app ./internal/httpserver -v`
Expected: FAIL because handlers are not wired to the worker response.

- [ ] **Step 3: Write minimal integration code**

```go
resp, err := h.app.Worker.Call(ctx, worker.Request{
    Action:    "run_affiliate_graph",
    SessionID: sessionID,
    Payload:   payloadBytes,
})
```

```go
writeJSON(w, http.StatusOK, viewmodel.FromWorkerResponse(resp))
```

- [ ] **Step 4: Re-run tests**

Run: `cd affiliate-ai-studio/go && go test ./internal/app ./internal/httpserver -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add affiliate-ai-studio/go
git commit -m "feat: wire go server to python worker"
```

### Task 7: Add final polish, verification, and local run instructions

**Files:**
- Modify: `affiliate-ai-studio/README.md`
- Modify: `affiliate-ai-studio/Makefile`
- Create: `affiliate-ai-studio/python/tests/test_runtime.py`

- [ ] **Step 1: Write the failing smoke tests**

```python
def test_worker_handles_run_affiliate_graph_action():
    runtime = bootstrap_runtime_for_tests()
    response = dispatch(runtime, {"action": "run_affiliate_graph", "session_id": "s1", "payload": {"product_url": "https://shop/p1", "platform": "TikTok", "locale": "id-ID", "style": "casual", "min_commission_rate": 0.1}})
    assert response["status"] == "ok"
```

```go
func TestHealthEndpoints(t *testing.T) {
    app := newStubApp()
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    rec := httptest.NewRecorder()
    NewHandler(app).Healthz(rec, req)
    if rec.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rec.Code)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd affiliate-ai-studio/python && python -m pytest tests/test_runtime.py -q`
Expected: FAIL.

Run: `cd affiliate-ai-studio/go && go test ./...`
Expected: FAIL because the health and ready endpoints are incomplete.

- [ ] **Step 3: Implement the final polish**

```make
test-go:
	cd go && go test ./...

test-python:
	cd python && python -m pytest

run:
	cd go && go run ./cmd/studio
```

- [ ] **Step 4: Run the full verification suite**

Run: `cd affiliate-ai-studio/go && go test ./...`
Expected: PASS.

Run: `cd affiliate-ai-studio/python && python -m pytest`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add affiliate-ai-studio
git commit -m "feat: finish affiliate ai studio"
```
