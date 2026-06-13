# ADR-0001 · 用 Go 单语言实现 agent-lab

- 状态：Accepted
- 日期：2026-06-13

## 背景

本仓库已同时存在 Go (根目录 `go.mod`) 与 Python (`lora/`、`affiliate-ai-studio/python/`) 项目。新建的 `agent-lab` 必须选定一种主语言。

## 决策

`agent-lab` 采用 **Go 单语言**。Python 体系仍留在 `lora/`，与 `agent-lab` 通过 OpenAI 兼容协议（HTTP）解耦。

## 理由

- **学习目标契合**：Agent 控制流的核心是循环、状态、并发、IO。Go 的并发原语 (goroutine + channel) 与显式错误处理直接服务于这些点。
- **去框架化**：Python 一旦引入会很难抵抗 LangChain / LangGraph / Pydantic AI 等生态。本项目的初衷就是不被框架带着走。
- **部署边界清晰**：Go 单二进制对工程化 (M8 HITL CLI、M9 trace 工具、M11 服务) 友好。
- **复用现有 module**：根目录已有 `module ai-learn-playground`，包路径 `ai-learn-playground/agent-lab/internal/...` 直接接入。

## 反方意见

- Python 在 LLM 生态周边 (tokenizer、tracing、eval) 工具更丰富。 → 这部分功能我们故意从零手写，工具丰富反而是减分项。
- 团队若以 Python 为主，门槛更高。 → 学习项目以"理解"为先，门槛是预期的一部分。

## 影响

- 所有 LLM 调用、工具、agent 编排、记忆与持久化均在 Go 中实现。
- 不引第三方 LLM SDK；标准库 + `database/sql` + 极少量第三方包 (例如 `modernc.org/sqlite` 或 `mattn/go-sqlite3`) 即可。
- 与 `lora/` 的协作方式：`lora/` 训出的模型若要被 `agent-lab` 使用，由 `lora/` 侧起一个 OpenAI 兼容 server，`agent-lab` 通过 base URL 接入。
