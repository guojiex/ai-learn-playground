# agent-lab 进度记录

项目: agent-lab — 从零手写 Agent 学习实验室
技术栈: 纯 Go + 本地大模型 (OpenAI 兼容协议)

---

## 2026-06-14 — M2 完成 (Tool Calling)

里程碑: M2 — 原生 function calling + 工具回环

### 已实现

**新模块: internal/tools/**
- `tool.go`: `Tool` 接口 (`Schema()` + `Invoke(ctx, args)`) + `Registry` 并发安全注册表 + `Schema()` helper + `ParseArgs()` helper.
- `product_lookup.go`: 按 id 精确或 query 模糊查询 `data/products/products.json`, 内置文件指纹缓存.
- `price_format.go`: 把 price + shipping + badges 拼成 `NT$690 · 現貨 · 限時免運` 格式.
- `platform_lint.go`: 校验 shopee_tw / pchome / momo 的字数 / 禁词 / 标签数, 返回 violations 列表.
- `slang_check.go`: 统计台湾电商黑话命中数与每千字密度.

**新模块: internal/agent/tooling.go**
- `Loop(ctx, client, registry, messages, opts)`: tool-calling 主循环.
  - 调 LLM → 拿到 tool_calls → `errgroup` 风格并发执行 → role=tool 回填 → 再调 LLM.
  - finish=stop 或无 tool_calls 时收敛; 达到 MaxSteps (默认 8) 报错.
  - 工具错误以 `{"error": ...}` JSON 回填, 模型可重试; 不打断循环.
  - 返回 `Result{FinalMessage, Steps, ToolCalls, Usage}`, 其中 `ToolCallRecord` 含 args / result / err / duration_ms.

**新 CLI: cmd/agent/main.go**
- `-m <message>` + `-data <dir>` + `-max-steps`, 调用 `agent.Loop`.
- 把每次 tool call 输出到 stderr (含耗时 / 摘要).

**Web 增量 (替换 /tools 占位)**
- `internal/web/handlers_tools.go`: `/tools` 页面 + `/api/tools/recent` + `/api/tools/invoke` (UI 试调用).
- `internal/web/tools_recent.go`: 进程内最近 50 条调用环形缓冲.
- `internal/web/server.go`: 引入 `ServerOption` + `WithToolRegistry`; 注入 registry 时启用 `/tools` 路由并把 nav 项的 disabled 取消.
- `templates/tools.html` + `static/tools.js` + `style.css` 中的 `.tools-page` / `.tool-card` / `.recent-list` 样式.
- `cmd/web/main.go`: 默认注册 4 个工具到 web server.

**测试**
- `internal/tools/tools_test.go`: 8 条用例覆盖 4 个工具的成功 / 失败路径与 Registry.
- `internal/agent/tooling_test.go`: 5 条用例 (stop / 单工具 / 并行 / 未知工具回填 / max-steps 守护).
- `internal/web/handlers_tools_test.go`: 5 条用例 (页面渲染 / invoke ok / unknown / recent buffer / 未注入时走占位).

### 文件清单

```
agent-lab/
├── cmd/
│   ├── agent/main.go                    (新, M2 CLI)
│   └── web/main.go                      (改, 注入 tools registry)
├── internal/
│   ├── tools/                           (新)
│   │   ├── tool.go
│   │   ├── product_lookup.go
│   │   ├── price_format.go
│   │   ├── platform_lint.go
│   │   ├── slang_check.go
│   │   └── tools_test.go
│   ├── agent/                           (新)
│   │   ├── tooling.go
│   │   └── tooling_test.go
│   └── web/
│       ├── server.go                    (改, ServerOption)
│       ├── nav.go                       (改, enabled map)
│       ├── handlers_tools.go            (新)
│       ├── handlers_tools_test.go       (新)
│       ├── tools_recent.go              (新)
│       ├── templates/tools.html         (新)
│       └── static/tools.js              (新)
└── data/products/products.json          (新, 3 条 sku 示例)
```

### 启动方式

CLI (需要支持 function calling 的真实模型, 例如 Qwen2.5-Instruct):

```powershell
$env:OPENAI_BASE_URL="http://127.0.0.1:8080/v1"
$env:OPENAI_API_KEY="sk-local"
$env:AGENTLAB_PROFILE="L"
go run ./agent-lab/cmd/agent -m "帮我为 sku_001 写一段蝦皮标题, 要带現貨/免運"
```

Web (Tools 面板可独立试调用所有工具, 不依赖 LLM):

```powershell
go run ./agent-lab/cmd/web
# 浏览器打开 http://127.0.0.1:8090/tools
```

### 验收

- [x] Registry 批量注册 / Schemas() 直接喂给 LLM
- [x] agent loop 最多 N 步 (默认 8), 超过报错
- [x] 工具错误以 role=tool 回填给模型
- [x] 多个 tool_calls 并发执行 (sync.WaitGroup), 顺序保持入参顺序
- [x] 单测覆盖回环逻辑, 不依赖真模型
- [x] /tools 面板列出 schema + 最近调用 + 试调用框

### 衔接

下一站候选:
- M3 (手写 ReAct, 与 M2 互为对照)
- M4 (在 M2 agent 上加短期 + 长期记忆 + SQLite)

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
