# Affiliate AI Studio Design

## Goal
Build a local-first web demo that turns the `LangChain x LangGraph` PDF into a runnable affiliate-business practice project. The demo should let the user experience prompt composition, structured output, RAG retrieval, tool calling, and LangGraph stateful routing through one end-to-end affiliate workflow.

## Product Shape
The product is a `Go 1.22` web server that exposes a browser UI and HTTP APIs, backed by a long-lived Python worker process that loads a local model and executes LangChain or LangGraph flows.

The default user journey is:

1. User enters a product URL or freeform product text.
2. The system normalizes the input and fetches product info from local sample data.
3. The system calls a commission tool and decides whether the product is worth promoting.
4. If the commission is too low, the flow exits early with an explanation.
5. If the commission is acceptable, the system retrieves policy and campaign knowledge from a local KB.
6. The system optionally compresses the retrieved context.
7. The local model generates structured affiliate copy.
8. The UI shows the result, supporting evidence, tool calls, and graph trace.

## User Experience
The product centers on a single page named `Affiliate AI Studio`.

The page contains:

- An input form for product URL, product text, target platform, locale, copy style, and minimum commission rate.
- A result panel showing structured copy output.
- A retrieval panel showing matched KB chunks and compressed context.
- A tool panel showing executed tools, inputs, and outputs.
- A trace panel showing LangGraph step-by-step decisions.

The page also exposes lab actions so the user can inspect one capability in isolation:

- `Prompt Lab`: prompt rendering and structured output.
- `RAG Lab`: chunking, retrieval hits, and compressed context.
- `Tool Lab`: run individual tools directly.
- `Agent Trace`: replay the end-to-end graph state transitions.

## Architecture
The architecture has three layers.

### Go Web Layer
The Go layer uses `net/http` and the Go 1.22 `ServeMux`.

Responsibilities:

- Serve HTML, CSS, and JavaScript.
- Validate HTTP requests.
- Manage session IDs and request correlation IDs.
- Start and supervise the Python worker process.
- Translate worker responses into stable HTTP JSON responses.

### Python AI Layer
The Python layer runs as a long-lived child process and communicates with Go over JSON lines through `stdin/stdout`.

Responsibilities:

- Load the local text generation model once.
- Load the local embedding model once.
- Build or load a local Chroma index.
- Run LangChain prompt and structured output flows.
- Run LangGraph affiliate routing.
- Return structured results, trace events, and error objects.

### Local Assets Layer
The local assets layer stores:

- Sample affiliate product records.
- Commission lookup fixtures.
- Policy and campaign documents.
- Few-shot prompt examples.
- Persisted vector index files.

## Module Boundaries
The Go side owns HTTP and rendering only. It must not implement prompt logic, retrieval logic, tool logic, or graph routing.

The Python side owns prompt logic, retrieval logic, tool logic, output schemas, and graph routing. It must not expose a separate public HTTP server.

## Core Flows
### Prompt Lab
Input is product info plus platform, locale, and style.

Output is:

- Rendered prompt text.
- Raw model output.
- Structured affiliate copy object.

This flow demonstrates prompt templates, chat prompts, and structured output.

### RAG Lab
Input is a natural-language question or generated retrieval query.

Output is:

- Retrieved chunks.
- Similarity-ranked results.
- Compressed context when enabled.
- The final context passed to generation.

This flow demonstrates chunking, embeddings, vector storage, retrieval, and compression.

### Tool Lab
Input selects a tool and a payload.

Output is:

- Tool description.
- Parsed input.
- Tool result.
- Tool error if applicable.

This flow demonstrates tool design and the importance of precise descriptions.

### Affiliate Graph
The graph keeps a shared state object that includes user input, product info, commission rate, retrieval evidence, structured copy, trace steps, and errors.

The default node order is:

1. `normalize_input`
2. `fetch_product_info`
3. `check_commission`
4. `route_by_commission`
5. `retrieve_policy_context`
6. `compress_context`
7. `generate_structured_copy`
8. `finalize_response`

If commission is below threshold, the graph exits through a rejection branch before retrieval or generation.

If a tool fails in a recoverable way, the error is written into graph state and returned as an observation so the graph can end gracefully with a helpful explanation.

## Data Contracts
### Request Contract
The primary run request includes:

- `product_url`
- `product_text`
- `platform`
- `locale`
- `style`
- `min_commission_rate`
- `enable_compression`
- `debug`

### Structured Output Contract
The generated affiliate copy includes:

- `title`
- `selling_points`
- `localized_hook`
- `risk_notes`

The final decision includes:

- `decision`
- `reason`
- `commission_rate`
- `copy`

### Trace Contract
Each trace step includes:

- `node`
- `summary`
- `input_snapshot`
- `output_snapshot`
- `status`

## Error Handling
The system should distinguish four failure classes:

1. `validation_error`
   Invalid HTTP payloads or unsupported options.
2. `worker_error`
   Python worker unavailable, crashed, or timed out.
3. `tool_error`
   Product or commission lookup failed.
4. `generation_error`
   Model generation or structured parsing failed.

All failures must be rendered into JSON responses with stable fields and surfaced in the UI without crashing the page.

## Testing Strategy
### Go
- Handler tests for request validation and response shaping.
- Worker client tests for JSON line protocol handling.
- Basic HTTP route tests for the studio page and health endpoints.

### Python
- Schema tests for structured output models.
- Tool tests for product lookup and commission routing.
- Retriever tests for KB hit formatting.
- Graph tests for low-commission rejection and accepted generation path.

### End-to-End
- A local integration test that starts the Go server, runs against the Python worker, and verifies the main affiliate flow returns structured JSON.

## Scope Decisions
The first version intentionally uses local fixture data instead of real affiliate APIs or real page scraping. This keeps the demo focused on LangChain and LangGraph behavior rather than external system integration.

The first version also keeps the UI server-rendered with light JavaScript instead of a full SPA, so more time goes into the AI workflow itself.

## Success Criteria
The first shippable version is successful when:

- The web page renders locally.
- The Python worker loads once and answers requests.
- The main affiliate flow works with local sample data.
- Prompt, RAG, tool, and graph behaviors are all inspectable in the UI.
- Go and Python tests cover the main happy path and the low-commission rejection path.
