# agent-lab 进度记录

项目: agent-lab — 从零手写 Agent 学习实验室
技术栈: 纯 Go + 本地大模型 (OpenAI 兼容协议)

---

## 2026-06-19 — M6 完成 (Planner-Executor: DAG 规划 + 分步执行 + 失败重规划)

里程碑: M6 — 把 "走一步看一步" (ReAct) 升级为 "先整体规划, 再分步执行", 支持失败重规划

### 已实现

**Plan 数据结构: internal/agent/plan_types.go (新)**
- `Plan` (goal + tasks[]), `Task` (id/name/depends/tool+args 或 agent+prompt), `TaskStatus` (pending/running/ok/fail/replan/skipped).
- `TaskResult` / `PlanRun` / `ReplanRecord`: 完整执行轨迹, 可 dump JSON 供 M9 trace.
- `Validate()`: ID 唯一 / 依赖存在 / tool-agent 互斥 / Kahn 拓扑排序检测环.
- `ReadyTasks(done)`: 返回当前可执行的 task (依赖全部完成).
- `TopoLevels()`: 把 DAG 分层, 同层可并行, 供 UI 列布局可视化.

**Planner: internal/agent/planner.go (新)**
- `Planner.Plan(ctx, goal)`: 一次 LLM 调用产出 Plan JSON, 解析失败最多重试 2 次 (复用 ReAct 的容错提取: 裸 JSON / ```json fenced / brace pair).
- `Planner.Replan(ctx, goal, original, failedTaskID, failReason, completedOutputs)`: 把 "进展 + 失败原因" 喂回 LLM 生成新计划.
- prompt 设计: 明确 JSON 协议, 注入可用工具列表, 给出典型流程 (kb_search → product_lookup → writer → platform_lint → composer).
- agent 任务的 prompt 支持 `{t1.output}` 引用上游任务输出.

**Executor: internal/agent/executor.go (新)**
- `Executor.Execute(ctx, plan, events)`: 按 DAG 依赖调度, 同层无依赖节点并发执行 (受限并发度 4).
- 两种子任务: tool (调 registry) / agent (调 LLM 生成文本).
- 上下文裁剪: 每个子任务只看自己依赖的输出 (substituteRefs), 不看全局历史.
- Replan: 子任务失败时调 Planner.Replan, 最多 maxReplan (2) 次; 失败节点的后续依赖标记为 skipped.
- `ExecEvent` SSE 事件: task_done / task_fail / replan / plan_done / plan_fail, 推送给 UI 实时刷新.

**CLI: cmd/plan/main.go (新)**
- `-m "<目标>"` 生成计划 → 打印 DAG (分层 + 依赖) → 执行 → 打印结果.
- `-dump <file>` 把执行轨迹 dump 成 JSON (供 M9 trace).
- 实时输出执行进度 + 最终结果摘要 (状态/耗时/重规划次数/总 token).

**Web UI: internal/web/**
- `server.go`: 新增 `WithPlannerExecutor` 选项 + `planner`/`executor` 字段; `/plan` + `/api/plan/generate` + `/api/plan/execute` 路由.
- `handlers_plan.go` (新): `GET /plan` 页面; `POST /api/plan/generate` (goal → Plan JSON + levels); `POST /api/plan/execute` (Plan → SSE 流式推送 ExecEvent).
- `templates/plan.html` + `static/plan.js` (新): Plan 面板, goal 输入 + 生成/执行按钮 + DAG 列布局可视化 (节点状态颜色) + 执行时间线.
- `static/style.css`: DAG 节点样式 (pending 灰 / running 蓝 / ok 绿 / fail 红 / replan 橙 / skipped 灰虚) + 时间线.
- `templates/layout.html`: footer 改 `M6 · planner`.
- `cmd/web/main.go`: 构造 Planner + Executor → `WithPlannerExecutor`.

**测试**
- `internal/agent/plan_types_test.go`: Validate (正常/重复ID/未知依赖/环/无tool-agent) / ReadyTasks / TopoLevels / parsePlan (裸JSON/fenced/带额外文本).
- `internal/agent/executor_test.go`: 全部成功执行 / 失败跳过依赖 / 并行执行 (<90ms for 2x50ms) / 引用替换 / Planner 重试.

### 启动方式

```bash
# 终端 A: fake 后端
go run ./agent-lab/scripts/fake-openai

# 终端 B: web
OPENAI_BASE_URL=http://127.0.0.1:18080/v1 OPENAI_API_KEY=sk-local AGENTLAB_PROFILE=L \
  go run ./agent-lab/cmd/web
# 浏览器 http://127.0.0.1:8090/plan
#   输入目标 → 生成计划 → 执行计划 (DAG 节点实时变色)

# CLI 方式:
OPENAI_BASE_URL=http://127.0.0.1:18080/v1 OPENAI_API_KEY=sk-local AGENTLAB_PROFILE=L \
  go run ./agent-lab/cmd/plan -m "为 sku_001 在小红书台湾发一篇上新文案" -dump /tmp/plan.json
```

### 验收

- [x] Planner 输出严格 JSON, 解析失败可兜底重新提问 (最多 2 次重试)
- [x] Executor 支持串行 + 受限并发 (依赖图允许并行的节点并发跑, 并发度 4)
- [x] 子任务失败时, Replan 至多 N 次 (默认 2), 超过则报最终失败
- [x] 整次 Run 能 dump 一份 "plan + 实际执行轨迹" 的 JSON (CLI `-dump`)
- [x] 单测覆盖: plan 验证 / 拓扑分层 / 并行执行 / 失败跳过 / 引用替换 / planner 重试
- [x] `go vet` / `go build` / `go test ./...` 全部通过
- [x] Web 冒烟测试: 手动构造 Plan → /api/plan/execute → SSE 推送 task_done + plan_done

### 衔接

下一站候选:
- M7 (Multi-Agent: 把 Plan 中的 agent: writer / agent: composer 落成真正的多 agent 消息流)
- M8 (HITL: 在 Plan 的关键节点引入人工审批)

---

## 2026-06-19 — M5 完成 (RAG: embedding + 向量检索 + kb_search 工具)

里程碑: M5 — 实现 RAG (Retrieval-Augmented Generation) 全链路: 文档切块 → embedding → 向量持久化 → cosine top-k 检索 → agent 工具调用 → Web 面板

### 已实现

**Embedding 客户端: internal/llm/embed.go (新)**
- `Embedder` 接口: `Embed(ctx, texts) → [][]float32` + `Dim() int`.
- `OpenAIEmbedder`: 调 OpenAI 兼容 `/v1/embeddings`, 自动探测维度, 与 chat client 分开 (可指向不同端口).

**Embedding 配置: internal/config/config.go (改)**
- 新增 `EmbedBaseURL` / `EmbedAPIKey` / `ModelEmbed` (环境变量 `AGENTLAB_EMBED_BASE_URL` / `AGENTLAB_EMBED_API_KEY` / `AGENTLAB_MODEL_EMBED`).
- `applyEmbedDefaults`: embed 后端为空时回退到 chat 后端; 默认模型 `bge-small-zh-v1.5`.

**Fake embedding server: scripts/fake-openai/embed.go (新)**
- `/v1/embeddings` 端点: 字符 bigram FNV 哈希 → 128 维向量 → L2 归一化.
- 确定性: 相同文本 → 相同向量; 共享 bigram 的文本 → 高余弦相似度, 足以演示 top-k 检索.

**向量持久化: internal/store/ (改)**
- `migrations.go`: 新增 `documents` 表 (id, source, chunk_index, text, embedding BLOB, metadata, created_at) + source 索引.
- `sqlite.go`: `PutDoc` / `LoadDocs` / `DeleteDocsBySource` / `CountDocs` / `ListDocSources`; float32 slice ↔ BLOB 编解码 (小端序, 紧凑).

**向量检索层: internal/memory/vector.go (新)**
- `VectorStore`: SQLite 持久化 + 内存暴力 cosine top-k (ADR-0005 起步策略, 零额外依赖).
- 启动时从 SQLite hydrate 全部向量到内存; `Add` / `DeleteBySource` write-through.
- `Search(query, k)`: 遍历全部向量做 cosine, 排序取 top-k.
- `cosineSim`: 通用余弦相似度 (自动处理未归一化向量).

**RAG 核心组件: internal/rag/ (新包)**
- `chunker.go`: `Chunk(text, cfg)` 按 rune 计数切分, 支持 overlap, 在句号/换行边界微调切点; `ChunkCount` 估算块数.
- `retriever.go`: `Retriever.Retrieve(ctx, query, k)` → embed query → VectorStore.Search; `RetrieveAndRender` 便捷组合; `Count/Dim/Sources` 委托.
- `render.go`: `Render(results)` 格式化成 system prompt 知识上下文块; `RenderToolResponse(query, results)` 生成 agent 可解析的 JSON.

**知识库搜索工具: internal/tools/kb_search.go (新)**
- `kb_search(query, k)`: agent 调用此工具检索知识库, 返回 top-k 文档块的 JSON.
- 写文案前 agent 可先 `kb_search(query="蝦皮標題字數限制")` 获取平台规则, 避免违规.

**数据导入工具: cmd/ingest/main.go (新)**
- `-dir <目录>`: 批量导入 .md/.txt; `-file <文件>`: 导入单文件; `-list`: 查看统计; `-delete <source>`: 删除.
- 流程: 读取 → `rag.Chunk` 切块 → 批量 `embedder.Embed` → `VectorStore.Add` 落库.
- 幂等: 同 source 先删旧再写新.

**平台规则文档: data/platform_rules/ (新)**
- `shopee_tw.md` / `pchome_tw.md` / `momo_tw.md` / `xiaohongshu.md`: 四个平台的标题/违禁词/hashtag/促销规则.

**Web UI 改造: internal/web/**
- `server.go`: 新增 `WithRetriever` 选项 + `retriever` 字段; `/knowledge` + `/api/knowledge` 路由; `enabledNav` / `loadTemplates` 接入 knowledge.html.
- `handlers_knowledge.go` (新): `GET /knowledge` 页面; `GET /api/knowledge` 统计 (sources/count/dim); `POST /api/knowledge` 检索 (query + k → top-k 结果 + 渲染上下文).
- `templates/knowledge.html` + `static/knowledge.js` (新): Knowledge 面板, 统计展示 + 搜索框 + 结果列表 (rank/score/source/text).
- `nav.go` + `placeholders()`: 新增 Knowledge 导航项与 M5 占位.
- `templates/layout.html`: 新增 book-open 图标; footer 改 `M5 · rag`.
- `handlers_tools.go`: 新增 `kb_search` 工具示例.
- `static/style.css`: 新增 Knowledge 面板样式.
- `cmd/web/main.go`: 构造 embedder → VectorStore → Retriever → 注册 kb_search → `WithRetriever`.

**测试**
- `internal/rag/chunker_test.go`: 短文本/空文本/overlap/边界/计数.
- `internal/rag/retriever_test.go`: top-k 检索准确性/空库/RetrieveAndRender/Render/RenderToolResponse.
- `internal/memory/vector_test.go`: 增删查/重开持久化/cosineSim 数学正确性.
- `internal/tools/kb_search_test.go`: 必填校验/返回 JSON 结构.
- `internal/web/handlers_knowledge_test.go`: 页面渲染/统计 API/搜索 API/未配置时占位.

### 启动方式

```bash
# 终端 A: fake 后端 (含 chat + embedding)
go run ./agent-lab/scripts/fake-openai

# 终端 B: 导入平台规则到向量库
OPENAI_BASE_URL=http://127.0.0.1:18080/v1 OPENAI_API_KEY=sk-local \
  AGENTLAB_DB_PATH=agent-lab/data/agent.db \
  go run ./agent-lab/cmd/ingest -dir agent-lab/data/platform_rules

# 终端 C: web
OPENAI_BASE_URL=http://127.0.0.1:18080/v1 OPENAI_API_KEY=sk-local AGENTLAB_PROFILE=L \
  go run ./agent-lab/cmd/web
# 浏览器 http://127.0.0.1:8090
#   /knowledge  搜索知识库, 查看检索结果与 score
#   /tools      试调用 kb_search 工具
#   /chat       agent 可自主调用 kb_search 获取平台规则后写文案
```

### 验收

- [x] embedding 后端可独立配置 (`AGENTLAB_EMBED_BASE_URL`), 为空时回退到 chat 后端
- [x] 文档切块 (500 字/块, 50 字 overlap) + 批量 embedding + SQLite 持久化
- [x] 向量库重启不丢 (hydrate from agent.db), 暴力 cosine top-k 检索准确
- [x] `kb_search` 工具返回结构化 JSON, agent 可在 ReAct/native 模式下自主调用
- [x] Knowledge 面板展示统计 + 搜索 + 结果 (rank/score/source/text)
- [x] 单测覆盖: chunker/retriever/vector store/kb_search/web knowledge
- [x] `go vet` / `go build` / `go test ./...` 全部通过

### 衔接

下一站候选:
- M6 (Planner-Executor: DAG 拆解 + 上下文裁剪, 依赖 M4 summarizer + M5 retriever)
- M9 (Trace: 复用 M5 的 store 做 trace 持久化)

---

## 2026-06-19 — M4 完成 (记忆: 短期摘要 + 长期 KV + SQLite)

里程碑: M4 — 把 M1 的短期滑窗升级为"滑窗+摘要"双层, 引入跨会话长期记忆 (KV), 用 SQLite 做单文件持久层

### 已实现

**SQLite 持久层: internal/store/ (新包)**
- 驱动选 `modernc.org/sqlite` (纯 Go, 无 CGO), 这是项目第一个外部依赖, 与 ADR-0005 一致.
- `sqlite.go`: `Open(path)` 打开/创建 `agent.db`, 设 WAL + busy_timeout + synchronous=NORMAL, 跑 migration.
- 单连接策略: `SetMaxOpenConns(1)` 让 `database/sql` 串行化所有操作, 彻底避免多连接竞争导致的 `SQLITE_BUSY`, 也让 `:memory:` 在池里共享同一库.
- `migrations.go`: 幂等 schema (`CREATE TABLE IF NOT EXISTS`): `memory_kv` / `conversations` / `conversation_messages` / `schema_meta`.
- KV 操作: `PutKV` (upsert ON CONFLICT) / `GetKV` / `DeleteKV` / `ListKV` / `Namespaces`.
- 会话操作: `SaveConversation` (upsert 会话行 + 全量替换消息, 100% 还原含 tool_calls) / `LoadConversation` / `ListConversations` / `DeleteConversation` (事务删两表).

**长期记忆: internal/memory/kv.go (新)**
- `KV` 在 store 之上加 namespace 约定: `seller:{id}` 分区, `tone` / `keywords` 为键.
- `SellerNamespace(sellerID)` 辅助函数.
- Put/Get/Delete/List/Namespaces 透传 store, nil store 时显式报错.

**摘要器: internal/memory/summarizer.go (新)**
- 把 M1 内联在 `EnsureBudget` 里的 LLM 摘要逻辑抽成独立 `Summarizer` 类型, 可单测、可被 M6 planner 复用.
- `Summarize(ctx, msgs)`: 调 LLM 压成 ≤300 字摘要; client 为 nil 或出错时走 `fallbackSummary` (拼接首句, 不丢关键字).
- `CompressInfo` 新增 `BeforeTokens` / `AfterTokens`, UI 摘要提示带上"压缩前/后 token 估算".
- `ShortTerm.EnsureBudget` 改用 `Summarizer`, 在每个压缩分支填上 token 估算.

**记忆工具: internal/tools/memory_get.go + memory_put.go (新)**
- `memory_get(namespace, key)`: agent 读长期记忆, 返回 `{found, value}`.
- `memory_put(namespace, key, value)`: agent 写长期记忆, value 必须是合法 JSON (校验后才入库).
- 两个工具自动注册到 web 的 tools registry, /tools 面板可试调用.

**Web UI 改造: internal/web/**
- `conversation.go`: `Conversation` 加 `SellerID` / `CreatedAt`; `ConversationStore` 加 `store *store.Store`, 支持 write-through (`EnablePersistence` / `Persist` / `Restore`); `New/Rename/Delete` 落库, 幂等删除.
- `server.go`: 新增 `WithStore` / `WithMemory` 选项; 注入 store 后 `EnablePersistence` + 启动 `hydrateConversations` (从 agent.db 把历史会话拉回内存, 实现"重启不丢历史"); 新增 `/memory` + `/api/memory` 路由; `enabledNav` / `loadTemplates` 接入 memory.html.
- `handlers_memory.go` (新): `GET /memory` 页面; `GET /api/memory` 按 namespace 折叠列出 KV; `DELETE /api/memory` 遗忘单条 (被遗忘权).
- `handlers_chat.go`: `chatAPIRequest` 加 `seller_id`; 发送时 `sellerHint` 注入 system prompt 引导 agent 用 memory_get/put; `summary` SSE 加 `before_tokens/after_tokens`; `finalizeChatSend`/`set_system`/`reset`/`load` 调 `persistConv` 落库; `switch` 返回 `seller_id`.
- `templates/memory.html` + `static/memory.js` (新): Memory 面板, namespace 折叠树, value 美化 JSON, 单条遗忘按钮.
- `templates/chat.html` + `static/chat.js`: composer 加 `seller` 输入框 (带 datalist); 会话列表标题前缀 `[卖家ID]`; switch/done 同步 seller_id; 摘要提示显示 token 估算.
- `templates/layout.html`: 新增 memory 图标 (数据库圆柱); sidebar-foot 改 `M4 · memory`.
- `nav.go` + `placeholders()`: 新增 Memory 导航项与占位.
- `cmd/web/main.go`: `-db` flag (默认 `AGENTLAB_DB_PATH` → `agent-lab/data/agent.db`); 打开 store → 建 KV → 注册 memory 工具 → `WithStore`+`WithMemory`; store 打开失败不致命, 退化为纯内存会话.
- `.gitignore`: 忽略 `agent-lab/data/agent.db{,-wal,-shm}`.

**测试**
- `internal/store/sqlite_test.go`: migration 幂等 (二次开库)、KV 增删改查 + 覆盖、**50 goroutine 并发写不破坏 schema**、会话 save/load/重开还原/全量替换/删除.
- `internal/memory/kv_test.go`: KV 读写列表删除 + 空值拒绝.
- `internal/memory/summarizer_test.go`: 摘要器调用 LLM / nil client fallback / 错误传播; `EnsureBudget` 越界触发摘要并填 token 估算 / 未越界不调 LLM.
- `internal/tools/memory_test.go`: memory_put/get 往返 / not found / 拒绝非 JSON value / 必填校验.
- `internal/web/handlers_memory_test.go`: /memory 渲染、未配置时占位、/api/memory 列表 + 遗忘.

### 文件清单 (相对 agent-lab)

```
├── go.mod / go.sum                           (改, +modernc.org/sqlite)
├── cmd/web/main.go                           (改, -db flag + store/kv/memory 工具装配)
├── internal/
│   ├── store/                                (新包)
│   │   ├── sqlite.go                         (Open + KV + 会话持久化)
│   │   ├── migrations.go                     (幂等 schema)
│   │   └── sqlite_test.go                    (新)
│   ├── memory/
│   │   ├── shortterm.go                      (改, CompressInfo 加 token 字段, EnsureBudget 用 Summarizer)
│   │   ├── summarizer.go                     (新, Summarizer 类型)
│   │   ├── kv.go                             (新, 长期 KV 门面)
│   │   ├── kv_test.go                        (新)
│   │   └── summarizer_test.go                (新)
│   ├── tools/
│   │   ├── memory_get.go                     (新)
│   │   ├── memory_put.go                     (新)
│   │   └── memory_test.go                    (新)
│   └── web/
│       ├── conversation.go                   (改, seller_id + write-through 持久化)
│       ├── server.go                         (改, WithStore/WithMemory + hydrate + /memory 路由)
│       ├── handlers_chat.go                  (改, seller_id + token 估算 + persist)
│       ├── handlers_memory.go                (新, /memory + /api/memory)
│       ├── handlers_memory_test.go           (新)
│       ├── handlers_tools.go                 (改, memory 工具示例)
│       ├── nav.go                            (改, Memory 导航/占位)
│       ├── templates/
│       │   ├── chat.html                     (改, seller 输入框)
│       │   ├── layout.html                   (改, memory 图标 + M4 foot)
│       │   └── memory.html                   (新)
│       └── static/
│           ├── chat.js                       (改, seller_id + token 估算)
│           ├── memory.js                     (新)
│           └── style.css                     (改, memory 面板样式)
└── .gitignore                                (改, 忽略 agent.db*)
```

### 启动方式

Web (默认落库到 `agent-lab/data/agent.db`, 重启不丢历史):

```bash
# 终端 A: fake 后端 (无需本地大模型)
go run ./agent-lab/scripts/fake-openai

# 终端 B: web
OPENAI_BASE_URL=http://127.0.0.1:18080/v1 OPENAI_API_KEY=sk-local AGENTLAB_PROFILE=L \
  go run ./agent-lab/cmd/web
# 浏览器 http://127.0.0.1:8090
#   /chat   composer 上方填 seller (如 A001), 发送后 agent 可用 memory_get/put
#   /memory 浏览/遗忘长期记忆 KV
```

换真实模型: 把 `OPENAI_BASE_URL` 指向 `llama-server` / `ollama` 即可. 自定义库路径: `-db /path/to/agent.db` 或 `AGENTLAB_DB_PATH`.

### 验收

- [x] `agent.db` 自动初始化, schema migration 幂等 (二次开库不报错)
- [x] 短期摘要触发后, SSE `summary` 事件带"摘要前 vs 摘要后" token 估算
- [x] 长期 KV 支持 `seller:{id}:tone` / `seller:{id}:keywords` 两个 namespace (memory_get/put 工具)
- [x] 整段 conversation 持久化并 100% 还原 (重启后 switch 仍能拿回全部消息 + system + seller_id)
- [x] 单测覆盖: 摘要被调用、KV 读写、50 goroutine 并发写不破坏 schema
- [x] `go vet ./...` / `go build ./...` / `go test ./...` 全部通过

### 衔接

下一站候选:
- M5 (RAG: embedding + chunking + 向量检索, 复用同一 agent.db)
- M6 (Planner-Executor: 上下文裁剪依赖 M4 的 summarizer)

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
