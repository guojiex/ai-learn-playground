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
- `web-studio/`: Studio v2 前端（React + Vite + Tailwind + React Flow），产物打到 `go/web/static/dist/`
- `assets/`: fixture products, commission data, and KB documents

## Quick Start

```bash
make install-python       # 安装 Python 依赖
make install-web          # 安装前端依赖（首次）
make build-web            # 构建前端到 go/web/static/dist/
make test                 # 跑 Go + Python 测试
make run                  # 启动 studio（:8080）
```

前端开发（HMR）：

```bash
# 终端 A
cd go && AFFILIATE_STUDIO_DEV=1 go run ./cmd/studio
# 终端 B
cd web-studio && npm run dev
# 访问 http://localhost:5173/studio
```

The first version works with fixture data out of the box. You can later replace the fallback model adapter with your own locally loaded model path.
