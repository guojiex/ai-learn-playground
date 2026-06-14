# agent-lab 进度记录

项目: agent-lab — 从零手写 Agent 学习实验室
技术栈: 纯 Go + 本地大模型 (OpenAI 兼容协议)

---

## 2026-06-14 — M3 完成 (手写 ReAct Agent)

里程碑: M3 — 不依赖原生 function calling 的 ReAct 协议, 与 M2 互为对照

### 已实现

**统一 Agent 接口: internal/agent/agent.go**
- `Agent` 接口: `Run(ctx, msg) (RunResult, error)` + `Mode()`
- 共享类型 `Step{kind, thought, action_name, action_args, observation, error, elapsed_ms}`
- 共享类型 `RunResult{final, steps, mode, elapsed, usage}`
- `Options{SystemPrompt, Model, Temperature, MaxTokens, MaxSteps}` 两种模式共用

**JSON 解析容错: internal/agent/parse.go**
- `ParseReAct(raw)` 按优先级尝试: 整块 JSON → 代码块 ```json...``` → 裸代码块 → 最外层 {...}
- 对每种候选再做 "单引号→双引号" 的宽容解析, 覆盖小模型常输出的 `{'name':'foo'}` 格式
- 协议校验: 必须有 `final` 或 `action`, 否则当解析异常处理
- 相关工具: `extractFenced / extractFirstBracePair / normalizeSingleQuotes / truncateForError`

**ReActAgent 主循环: internal/agent/react.go**
- `ReActSystemPrompt(baseSystem, toolNames)` 动态注入工具列表
- 主循环: 调 LLM → ParseReAct → final 收敛 或 action 调用工具 → 把 observation 以 user 角色追加
- 解析失败策略: 第一次发 "你的输出不符合 JSON 协议" 让模型重试; 第二次降级把原文当 final
- MaxSteps 守护: 超过上限返回 `ErrMaxSteps`
- `invokeTool`: 未知工具 / args 解析失败统一返回 JSON 错误, 让模型自行重试

**NativeAgent 包装: internal/agent/native.go**
- 把 M2 `Loop()` 包装成 `Agent` 接口, 方便同一份 Web/CLI 在两种模式间切换对照

**CLI: cmd/agent/main.go 加 `--mode` 切换**
- 默认 `native` (M2 原生 function calling), 传 `--mode=react` 走 M3 JSON 协议
- 支持 `--temperature` / `--max-tokens` / `--max-steps` 全流程共享参数

**Web Chat UI 改造: internal/web/handlers_chat.go + templates/chat.html + static/chat.js**
- `POST /api/chat` 新增 `mode` 字段: `native`(默认) 或 `react`
- `handleChatSendReact`: 调用 `ReActAgent.Run()`, 对每个 step 发 SSE `step` 事件, 最后发 `final` + `done`
- `handleChatSendNative`: 保留原有的 ChatStream 增量流式
- `chat.html` 作曲家选项区: mode 下拉 + temperature 数字 + max_tokens 数字
- `chat.js` 新增 `addStepCard()` 渲染 step (thought / action / observation / error 分区), 处理 `start / step / final / delta` 事件
- `style.css` 新增 `.composer-options` 工具条样式和 `.msg.step-card` 反应式卡片样式

**测试**
- `internal/agent/react_test.go`: 多条用例 (final 直接收敛 / action 调用工具 / max-steps / 解析失败降级 / 未知工具)
- `internal/web/handlers_chat_test.go`: 已有的 send 流式用例对两种模式回归

### 文件清单 (相对 agent-lab)

```
├── cmd/agent/main.go                     (改, 加 --mode=react|native)
├── internal/agent/
│   ├── agent.go                          (新, Agent 接口 + 共享类型 + Options)
│   ├── parse.go                          (新, JSON 提取 + 代码块容错)
│   ├── react.go                          (新, ReActAgent 主循环)
│   ├── native.go                         (新, 包装 tooling.go Loop 为 Agent)
│   └── react_test.go                     (新)
└── internal/web/
    ├── handlers_chat.go                  (改, 路由 react / native, 发 step SSE)
    ├── templates/chat.html              (改, composer-options 区)
    └── static/
        ├── chat.js                       (改, mode 参数 + step/final 事件渲染 + escapeHtml)
        └── style.css                     (改, composer-options + step-card 样式)
```

### 启动方式

CLI:

```powershell
$env:OPENAI_BASE_URL="http://127.0.0.1:8080/v1"
$env:OPENAI_API_KEY="sk-local"
$env:AGENTLAB_PROFILE="L"
# M3 手写 ReAct (JSON 协议, 不需要 function calling)
go run ./agent-lab/cmd/agent --mode=react -m "为 sku_001 写一段標題"
# M2 对照
go run ./agent-lab/cmd/agent --mode=native -m "为 sku_001 写一段標題"
```

Web:

```powershell
go run ./agent-lab/cmd/web
# http://127.0.0.1:8090/chat  → composer 上方的 mode 下拉切换 native / react
```

### 验收

- [x] `Agent` 接口: NativeAgent / ReActAgent 都能 Run 并返回 RunResult
- [x] ParseReAct 对代码块 / 单引号 / 裸 JSON 三种格式都能解析
- [x] ReActAgent 能调用 tools registry 并把 observation 回填
- [x] CLI `--mode=react` 与 `--mode=native` 都能跑通
- [x] Web Chat 切换 react 模式后, step 卡片逐条渲染, 最后显示 final
- [x] `go vet ./...` / `go build ./...` / `go test ./...` 全部通过

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

## 2026-06-14 — M2 补丁 (Tooling UI + 会话删除)

### Bug 修复

**工具面板 UI (handlers_tools.go / tools.html / tools.js / style.css)**
- 试调用 textarea placeholder 写死了 product_lookup 示例, 每个工具现配自己的最小示例 (`toolExamples` map).
- 新增「填示例」按钮, 一键把工具示例复制到 textarea.
- `.invoke-result:empty` 加 `display:none`, 无结果时不撑出黑边.
- JSON Schema textarea 加 `box-sizing:border-box; max-width:100%`, 长行不再贴边.
- `GET /api/conversations` 加 `Cache-Control:no-store` 防浏览器缓存旧列表.
- 前端 `loadConversations()` 加递增序号 (`loadSeq`), 丢弃陈旧响应避免并发乱序覆盖.

**会话删除修复 (conversation.go / handlers_chat.go)**
- `Conversation` 结构体没有 json tag, Go 默认输出大写字段名 (`ID`/`Title`/`UpdatedAt`).
- 前端 JS 用 `c.id` / `c.title` 取值, 永远拿到 `undefined`.
- `JSON.stringify({..., conversation_id: undefined})` 会丢弃该键, 删除请求体里只有 `{"action":"delete"}`.
- 修复: 给 `Conversation` 加 `json:"id"`, `json:"title"`, `json:"updated_at"` tag.
- 幂等删除: server 对不存在的 id 仍返回 `ok:true` (不返回 404), 前端不再依赖 existed 字段.

### 提交

- `a7d4bb3` fix(web): 会话 JSON 键名改为小写, 删除按钮不再丢失 conversation_id; 列表 API 加 no-store 防缓存; 前端 loadConversations 加请求序号避免并发响应乱序

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
