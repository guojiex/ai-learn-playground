# M0 · 项目骨架 + 本地 LLM Client + Web UI 脚手架

**前置**：无  
**推荐档**：S / L  
**预计代码量**：~600 行 Go (CLI ~300 + Web 脚手架 ~300)

## 学习目标

- 把"调用本地 OpenAI 兼容 server"这件事从命令行下沉到一个最小 Go 客户端。
- 形成一个**所有里程碑都能复用**的骨架：配置、日志、超时、重试、流式。
- 跑通 `cmd/chat`：`go run ./agent-lab/cmd/chat -m "你好"`，得到一段流式输出。
- 起一个最小 Web UI：`cmd/web` 用 `embed.FS` + `html/template` + SSE，能在浏览器里完成一次完整流式问答，并预留左侧面板导航。

## 关键概念

- **OpenAI 兼容协议** (`/v1/chat/completions`) 的字段：`model` / `messages` / `temperature` / `stream` / `tools`。
- **流式 (SSE)**：`data: {...}\n\n` + `data: [DONE]`，逐 chunk 解析 `delta`。
- **超时**：`context.WithTimeout` + `http.Client{Timeout: ...}` 的差别。
- **重试**：仅对 5xx / 网络错误重试；对 4xx 立刻报错。
- **Server-side SSE**：`text/event-stream` 响应，按 `data: ...\n\n` 写帧并 `Flusher.Flush()`。
- **客户端 EventSource / ReadableStream**：浏览器侧逐帧渲染 token；用户点击"停止"即关闭连接，触发 server `ctx.Done`。

## 要写的代码

```
agent-lab/
├── go.mod                       # 复用根 go.mod, 不新建
├── cmd/
│   ├── chat/
│   │   └── main.go              # CLI 入口, 解析 -m / --stream
│   └── web/
│       └── main.go              # 启动 internal/web 的 HTTP server
├── internal/
│   ├── config/
│   │   └── config.go            # 加载 env: OPENAI_BASE_URL / API_KEY / AGENTLAB_*
│   ├── llm/
│   │   ├── client.go            # LLMClient 接口与 Message/ChatRequest/Response
│   │   └── openai.go            # openai-compatible 实现 (Chat / ChatStream)
│   └── web/
│       ├── server.go            # http.Server 装配 + 路由
│       ├── handlers_chat.go     # GET /chat 渲染模板, POST /api/chat 转发 SSE
│       ├── handlers_misc.go     # GET / -> /chat, /healthz, 占位面板
│       ├── templates/
│       │   ├── layout.html      # 左侧导航 + 主插槽
│       │   ├── chat.html        # Chat 面板
│       │   └── placeholder.html # "M? 启用后开放"占位
│       └── static/
│           ├── style.css        # 单文件手写
│           └── chat.js          # fetch + SSE 渲染 + 停止
```

依赖：标准库 + `encoding/json`。**不引第三方 SDK**；本里程碑的核心就是亲手写一遍。

## 业务表现

```bash
export OPENAI_BASE_URL=http://127.0.0.1:8080/v1
export OPENAI_API_KEY=sk-local
export AGENTLAB_PROFILE=L

go run ./agent-lab/cmd/chat -m "用一句话介绍今治毛巾"
# (流式逐字输出)

go run ./agent-lab/cmd/chat -m "..." --no-stream
# (一次性输出 + token 用量)
```

Web UI：

```bash
go run ./agent-lab/cmd/web                # 默认 listen 127.0.0.1:8090
# 浏览器打开 http://127.0.0.1:8090/
# 在 Chat 面板输入消息, token 流式刷出; 顶部显示当前 model 与 base URL
```

## UI 增量 (M0)

- **导航**：Chat / Tools / Plan / Multi-Agent / Approvals / Traces / Router / Settings；除 Chat 外都是占位页。
- **Chat 面板**：
  - 顶部：当前 profile / model / base URL（来自 `internal/config`）。
  - 中部：单会话消息流，user 与 assistant 气泡区分；assistant 流式 token 实时拼接。
  - 底部：textarea + 发送 + 停止按钮；Enter 发送、Shift+Enter 换行。
- **状态**：M0 的会话仅活在浏览器内存 (页面刷新即丢)，与 server 端会话化要到 M1。

## 验收标准

- [ ] `cmd/chat` 在 S 档与 L 档下都能跑通 (只换 `OPENAI_BASE_URL` 与 `AGENTLAB_MODEL_CHAT`)。
- [ ] 流式与非流式两种模式都可用，参数化切换。
- [ ] 服务返回 5xx 时自动重试 3 次，间隔指数退避；4xx 立刻报错并打印响应体。
- [ ] `Ctrl-C` 在流式中能立刻中断 (HTTP request 受 ctx 取消)。
- [ ] 没有第三方 LLM SDK 依赖。
- [ ] `cmd/web` 起服后浏览器可见 Chat 页面，并完成一次完整流式问答 (走 fake-openai 即可)。
- [ ] Chat 页面"停止"按钮关闭流后，server 日志能看到 `ctx canceled`，backend 不再继续生成。
- [ ] 静态资源经 `embed.FS` 注入，单二进制 (`go build -o agent-web ./agent-lab/cmd/web`) 可独立运行。
- [ ] 至少 1 条单测覆盖 `POST /api/chat` 的 SSE 中转行为 (复用 M0 fake server 思路)。

## 进阶练习

1. 加 `--temperature` / `--max-tokens` / `--system` 三个 flag。
2. 把请求/响应打到一个轮转 JSON log，便于后续 trace。
3. 写一个 fake 后端 (本地起 HTTP server)，让单测可以跑 `Chat` / `ChatStream`，不用真起模型。
4. 在 Chat 页面顶部显示一段实时"延迟/吞吐"指示 (上一次响应 token/s)。

## 衔接

下一站 [M1](m1-chat-loop.md)：把单次调用扩展成多轮对话；UI 加会话列表、摘要触发提示、角色卡编辑。
