# agent-lab 进度记录

项目: agent-lab — 从零手写 Agent 学习实验室
技术栈: 纯 Go + 本地大模型 (OpenAI 兼容协议)

---

## 2026-06-14 — M1 完成

里程碑: M1 — 多轮对话 + Prompt 工程

### 已实现

**CLI (cmd/chat/main.go)**
- REPL 模式，连续对话，多轮历史保存在内存
- 命令: `:reset` / `:system [text]` / `:save [path]` / `:load <path>` / `:history` / `:quit`
- 流式输出, Ctrl-C 中断
- 支持 `-m` 首条消息 + `-persona` 加载角色卡
- 支持 `--no-stream` 关闭流式

**新模块: internal/memory/shortterm.go**
- `ShortTerm`: system prompt + messages history
- `EstimateTokens`: 中文按字符估算 token (中文 2/3 + 英文 1/4)
- `EnsureBudget`: 超预算时先滑窗, 再调 LLM 摘要
- `SaveToFile` / `LoadFromFile`: JSON 持久化

**新模块: internal/prompt/ (persona.go, templates.go)**
- `Default()` 台湾电商文案助理角色卡
- 支持从 personas/ 目录加载自定义角色卡
- `QuestionPrompt` / `StyleHint` 工具函数

**Web 改进: internal/web/**
- `conversation.go`: `ConversationStore` 管理多会话, 支持 new/rename/delete/switch/load
- `handlers_chat.go`: 统一 action 路由 (send/new/switch/rename/delete/set_system/reset/export/load)
- `server.go`: 新增会话管理, 新增 `/tutorial` 路由
- `chat.html`: 左侧会话列表 + 角色卡编辑区
- `chat.js`: 多会话 UI, 导出/导入 JSON, 摘要提示气泡
- `tutorial.html`: 完整设计教程页

**tests**
- 现有 handlers_chat_test.go 保持 M0 测试, M1 暂未新增测试

### 文件清单

```
agent-lab/
├── cmd/chat/main.go                      (REPL 多轮)
├── internal/
│   ├── memory/shortterm.go               (会话管理)
│   ├── prompt/persona.go                 (角色卡)
│   ├── prompt/templates.go               (prompt 工具)
│   ├── prompt/personas/tw-ecom-copywriter.md
│   ├── web/conversation.go              (会话 store)
│   ├── web/handlers_chat.go            (API handlers, action 路由)
│   ├── web/server.go                  (装配路由 + /tutorial)
│   └── web/static/
│       ├── chat.js                       (M1 UI)
│       ├── tutorial.html               (设计教程页)
│       └── style.css                     (含 chat-layout 样式)
├── internal/config/config.go                (配置)
├── internal/llm/*.go                      (协议)
├── testserver.go
└── scripts/fake-openai/main.go           (echo LLM server 测试用)
```

### 启动方式

```bash
# 终端 A: fake LLM server
go run ./agent-lab/scripts/fake-openai

# 终端 B: Web UI
set OPENAI_BASE_URL=http://127.0.0.1:18080/v1
set OPENAI_API_KEY=sk-local
set AGENTLAB_PROFILE=L
go run ./agent-lab/cmd/web
# 浏览器打开 http://127.0.0.1:8090

# 终端 C: CLI REPL
go run ./agent-lab/cmd/chat
```

### 验收

- [x] Web: :reset / :save / :load 工作
- [x] 角色卡可编辑, 保存即生效
- [x] 上下文超出预算时自动裁剪或摘要
- [x] 会话可导出为 JSON 导入
- [x] 左侧会话列表: 新建/切换/重命名/删除
- [x] 设计教程页可访问 /tutorial

---

## 2026-06-14 — M0 (初始骨架)

里程碑: M0 — 最小骨架

### 已完成
- CLI: 单轮对话, 流式输出
- Web: Chat 单会话, 流式气泡
- fake-openai server
- 设计文档: 00-overview 到 06-ui 全部
- 测试: handlers_chat_test.go, openai_test.go

### 里程碑依赖关系
M1 ← M2 / M3 (M4 ← M5 ← M6 ← M7 ← M8 ← M9 ← M10 ← M11

下一个里程碑: M2 (Tool calling) 或 M3 (手写 ReAct)
