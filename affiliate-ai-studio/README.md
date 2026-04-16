# Affiliate AI Studio

`Affiliate AI Studio` is a local-first practice project for `LangChain`, `LangGraph`, prompt engineering, structured output, RAG, and agent routing in an affiliate marketing scenario.

## Highlights

- Go 1.22 web shell with `net/http` and `ServeMux`
- Python worker process for prompt, RAG, tool, and graph execution
- Local fixture data for products, commissions, and policy documents
- Browser UI that shows generated copy, retrieval evidence, tool calls, and execution trace

## Project Layout

- `go/`: web server, handlers, worker client, templates, and static assets
- `python/`: worker runtime, prompt lab, retriever, tools, and graph
- `assets/`: fixture products, commission data, and KB documents

## Quick Start

```bash
make install-python
make test
make run
```

The first version works with fixture data out of the box. You can later replace the fallback model adapter with your own locally loaded model path.
