# 00 · 概览

## 学什么

`agent-lab` 是一个**从零手写**的 Agent 学习项目。目标是把"Agent"这个词拆开成可以自己实现的组件：

- LLM 客户端 (chat / stream / embedding)
- 工具调用 (function calling 协议、JSON schema、并发与回填)
- 控制循环 (ReAct、Plan-and-Solve)
- 记忆 (短期滑窗 / 摘要 / 长期 KV / 向量检索)
- 协作 (Planner-Executor、Multi-Agent、Human-in-the-Loop)
- 工程化 (trace、评测、模型路由、降级)

所有组件都用 Go 实现，每一步都对应一个可运行的子命令和一段电商文案业务场景。

## 为什么从零手写

- **可解释**：当 Agent 行为异常时，能定位到自己写的某行代码，而不是黑盒框架。
- **去框架化**：理解 LangGraph / Agents SDK 这类框架的底层抽象，而不是被它们绑住。
- **小步快跑**：每个里程碑产物都可独立运行、可观测、可评测。

## 为什么全本地模型

- 学 Agent 控制流时会跑大量循环，云端 API 成本与限流都让节奏卡顿。
- 本地模型可以观察到"模型能力天花板"对 Agent 行为的影响，是个有价值的学习面。
- 通过统一的 OpenAI 兼容协议接入，未来想切到云端只是改一个 `OPENAI_BASE_URL`。

## 整体形态

```
┌──────────────────────────────────────────────────────────────┐
│                       agent-lab (Go)                         │
│                                                              │
│  cmd/web  cmd/chat  cmd/agent  ...                           │
│       \      |        /                                      │
│        ▼     ▼       ▼                                       │
│        agent ──▶ tools                                       │
│          │         │                                         │
│          ▼         ▼                                         │
│        memory   llm.Client (OpenAI 兼容)                     │
│          │         │                                         │
│          ▼         ▼                                         │
│        SQLite   http://127.0.0.1:PORT/v1                     │
│                    │                                         │
└──────────────────────────┼───────────────────────────────────┘
                           ▼
              llama.cpp / Ollama (本地推理)
                           │
                           ▼
                    Qwen2.5-{1.8B,3B,7B,14B}
```

- **Go 进程**只负责 agent 编排、工具、记忆、评测，并对外暴露 CLI (`cmd/chat` 等) 与 Web UI (`cmd/web`)。
- **本地推理后端**只负责吐 token。
- 两者通过 OpenAI 协议解耦，agent 代码层对模型与硬件无感。
- CLI 与 Web 共用 `internal/*` 业务层，二者并列存在，没有"哪一个是主入口"的主从关系。

## 与同仓库 lora/ 的关系

`lora/` 训练出来的电商文案模型是**素材与领域感**的来源，它的：

- 数据格式 (`tw_affiliate.jsonl`)
- 评测协议 (PPL / 黑话命中率)
- 基模 (Qwen1.5-1.8B-Chat)

在 `agent-lab` 里都会被借鉴，但 `agent-lab` 不会强依赖 `lora/` 的产物。如果你想把 LoRA 适配后的模型当成一个 specialist 接入，那是 M10 模型路由的"加分练习"，不是必修。

## 不做什么

- 不做云端 API 接入 (Doubao / OpenAI / Claude / Gemini)。
- 不做生产级部署 (k8s、网关、鉴权)；只到"本地工程化"。
- 不做训练/微调；那部分在 `lora/`。
- 不引入 Node 前端工具链 (Vite / React / Tailwind 等)；UI 用 Go `html/template` + 原生 fetch + SSE 自己写。
