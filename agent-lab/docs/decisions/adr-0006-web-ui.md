# ADR-0006 · Web UI 作为演示主界面

- 状态：Accepted (修订 ADR-0001 / 00-overview 中"CLI / HTTP only"的口径)
- 日期：2026-06-13

## 背景

M0 落到命令行后，立刻显现一个问题：多轮对话、HITL approval 队列、multi-agent 消息流、trace 时间线，这些都是**面状信息**，CLI 展示成本高且学习反馈弱。

## 决策

- 引入一个 Web UI 作为**演示主界面**：`cmd/web`，单 Go 进程，端口默认 8090。
- **不引入 Node 工具链**：Go `html/template` + 原生 fetch + ReadableStream/SSE + 极少量手写 CSS/JS。
- **CLI 保持完整**：`cmd/chat` 等 CLI 工具是测试与脚本主接口，UI 不取代它们。
- 静态资源用 `embed.FS` 打包进二进制。

## 理由

- **学习反馈**：Agent 的有趣行为是"多步、多角色、多工具、并发"，用 UI 能在一屏内看见因果。
- **零外部前端构建**：HTMX-风格 + 原生 SSE 已足够；引入 Vite/React 会让本项目失去"单二进制"特质。
- **演进友好**：每个里程碑只往 UI 里加一个面板，不需要全局重构。

## 反方意见

- HTML 模板 + 原生 JS 会在交互复杂时显得笨重。 → 真到那一步再评估；本项目的交互复杂度被 11 个里程碑明确限定。
- 重复实现 CLI 和 UI 两套入口。 → 共用 `internal/*` 业务层，差异只在适配层 (cmd/chat 处理 stdin/stdout，cmd/web 处理 HTTP)，重复成本可控。

## 影响

- 新增目录 `agent-lab/cmd/web` 与 `agent-lab/internal/web`。
- 每个里程碑文档"业务表现"小节增加一条"UI 表现"，但 CLI 表现不删除。
- 后续 M4 持久化、M8 HITL、M9 trace、M7 multi-agent 都会在同一 Web 里挂面板。
- 端口与 base URL 约定：UI = 8090，本地推理 backend = 8080，Embedding = 8081。
