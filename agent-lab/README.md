# agent-lab

从零手写一个 Agent 学习实验室。**纯 Go**、**全程使用本地大模型**、**不依赖任何云端 API**。业务场景统一围绕**台湾电商文案**，沿 11 个里程碑从单轮对话推进到 multi-agent + planner + HITL + 评测 + 路由。

本目录的设计文档已全部完成，代码按里程碑逐步落地 (M0–M4 已完成)。

## 阅读顺序

1. [docs/00-overview.md](docs/00-overview.md) — 这是什么、为什么、整体形态
2. [docs/01-roadmap.md](docs/01-roadmap.md) — 11 个里程碑总表与依赖关系
3. [docs/02-architecture.md](docs/02-architecture.md) — 分层架构与关键抽象
4. [docs/03-glossary.md](docs/03-glossary.md) — 术语对齐
5. [docs/04-local-model-stack.md](docs/04-local-model-stack.md) — 硬件分档、模型选型、后端起服流程
6. [docs/05-ecom-scenario.md](docs/05-ecom-scenario.md) — 电商文案业务定义
7. [docs/06-ui.md](docs/06-ui.md) — Web UI 设计与里程碑增量
8. [docs/milestones/](docs/milestones) — 逐里程碑学习材料
9. [docs/decisions/](docs/decisions) — 关键设计决策 (ADR)

## 最短上手路径

1. 按 [docs/04-local-model-stack.md](docs/04-local-model-stack.md) 起一个本地 OpenAI 兼容 server (llama.cpp 或 Ollama)。
2. 进入 [docs/milestones/m0-skeleton.md](docs/milestones/m0-skeleton.md)，跑通 `cmd/chat`。
3. 按 [docs/01-roadmap.md](docs/01-roadmap.md) 顺序往下推。

## 与本仓库其他项目的关系

- `lora/`：Python LoRA 微调实战，提供电商文案领域语料与基模 (Qwen1.5-1.8B-Chat)。`agent-lab` 复用其语料感觉与评测思路，但**不直接依赖**它。
- `affiliate-ai-studio/`：另一个独立子项目，与本目录无依赖。

## 状态

| 阶段 | 状态 |
|------|------|
| 设计文档 | 完成 |
| M0 代码 | 完成 (`cmd/chat` + `internal/llm` + `internal/config` + fake server) |
| M0 Web UI | 完成 (`cmd/web` + `internal/web`, SSE 流式 + 占位面板) |
| M1 代码 | 完成 (`internal/memory` + `internal/prompt` + 多会话 / 角色卡 / 摘要压缩) |
| M2 代码 | 完成 (`internal/tools` + `internal/agent` + `cmd/agent` + Web `/tools`) |
| M3 代码 | 完成 (`internal/agent` ReAct + `--mode=react` + step 卡片) |
| M4 代码 | 完成 (`internal/store` SQLite + 长期 KV + Summarizer + Web `/memory` + 会话持久化) |
| M5–M11 代码 | 未开始 |

## M0 快速跑通

起一个 fake server (无需本地大模型)：

```powershell
go run ./agent-lab/scripts/fake-openai
```

另开一个终端：

```powershell
$env:OPENAI_BASE_URL="http://127.0.0.1:18080/v1"
$env:OPENAI_API_KEY="sk-local"
$env:AGENTLAB_PROFILE="L"
go run ./agent-lab/cmd/chat -m "你好"
go run ./agent-lab/cmd/chat -m "再说一次" -no-stream
```

换到真实本地模型时把 `OPENAI_BASE_URL` 指向 `llama-server` / `ollama` 即可，详见 [docs/04-local-model-stack.md](docs/04-local-model-stack.md)。

## M0 Web UI 体验

一键拉起 fake 后端 + Web，然后浏览器打开 `http://127.0.0.1:8090/`：

```powershell
powershell -ExecutionPolicy Bypass -File .\agent-lab\tools\run.ps1 demo-web
```

或分两步：

```powershell
# 终端 A
go run .\agent-lab\scripts\fake-openai

# 终端 B
$env:OPENAI_BASE_URL="http://127.0.0.1:18080/v1"
$env:OPENAI_API_KEY="sk-local"
$env:AGENTLAB_PROFILE="L"
go run .\agent-lab\cmd\web                 # 默认 127.0.0.1:8090
```

UI 当前包含：Chat (多轮对话 / 流式 / native+react 模式 / seller 切换) / Tools (工具 schema + 试调用) / Memory (长期记忆 KV 浏览, M4) / Settings (运行时配置) / 其余面板占位 (`Plan` / `Multi-Agent` / `Approvals` / `Traces` / `Router` 等里程碑陆续启用)。会话与长期记忆默认落 `agent-lab/data/agent.db` (M4)，重启不丢历史。
