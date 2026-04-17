# Studio v2 · Graph Workbench 设计稿

- **日期**：2026-04-17
- **作者**：@jiexin.guo（AI pairing）
- **状态**：Draft · 等确认
- **关联**：`2026-04-15-affiliate-ai-studio-design.md`

---

## 1. 动机

当前 `go/web/templates/studio.html` 是表单 + 4 个 `<pre>JSON</pre>` 面板。信息是全的，但：

1. 看不出"Graph"是怎么跑的，仅 learn 页嘴上说"StateGraph 节点/边"，studio 里没对应可视化；
2. 结构化结果以 JSON 展示，用户体会不到"LLM 生成 marketing copy"的最终形态；
3. 错误/空态/加载态都是单行文字，与已经加上的 tool_error 上下文不呼应。

目标：把 studio 页升级成 **Graph Workbench**——中间一张实时高亮的 DAG，左边输入，右边结构化卡片 + 详情抽屉，让用户"看见每一次 LangGraph run"。

## 2. 非目标

- 不做多会话历史/对比（那是方向 C）
- 不做 KPI 仪表盘（那是方向 D）
- 不改后端 Graph 节点语义，**后端 JSON schema 不变**（避免牵连 Python 侧测试）

## 3. 技术选型

| 层 | 选择 | 理由 |
|---|---|---|
| 前端框架 | **React 18 + TypeScript + Vite** | 用户指定；生态最顺；DAG 可视化组件成熟 |
| DAG 可视化 | **React Flow (`@xyflow/react`)** | MIT、免费、动画/自定义节点成熟、包小（gzip < 60KB） |
| 样式 | **Tailwind CSS + shadcn/ui 风格手写组件** | 不引入全套 shadcn（避免 CLI 依赖），但沿用其 token 体系；暗色优先 |
| 状态 | React `useReducer` + `zustand`（单 store，够用） | 不过度引入 redux |
| 图标 | **lucide-react** | 轻、风格统一 |
| 构建输出 | `web/static/dist/` 下的 hashed assets + `index.html` | Go 继续 `http.FileServer`，**不用** `go:embed` 第一版 |
| 开发体验 | `vite dev` 起在 5173，代理 `/api/*`、`/learn`、`/healthz` 到 Go 8080 | Go 服务仍负责 API + 模板；前端独立热更新 |
| 生产部署 | `vite build` 产物放 `web/static/dist/`，Go 模板 `studio.html` 改为 shell，插入 Vite manifest 指定的 JS/CSS | 一条 `make build-web` 打包；Go `go run` 直接能跑 |

**不选 Next.js / Remix**，因为当前 Go 已负责路由 + 模板，强上全栈 React 会冲突；方案维持"Go 后端 + React SPA island"，老页面（/learn）不动。

## 4. 目录结构改动

```
affiliate-ai-studio/
├── go/
│   └── web/
│       ├── templates/
│       │   ├── studio.html     # 改为 Vite shell（见 §6）
│       │   └── learn.html      # 不动
│       └── static/
│           ├── app.css         # 保留，只服务 learn 页共享样式
│           ├── learn.css       # 不动
│           ├── learn.js        # 不动
│           └── dist/           # ← Vite build 输出（新建；gitignore 里加）
│               ├── index.html  # 不直接访问，仅供 Go 读取 manifest
│               ├── assets/
│               │   ├── studio-<hash>.js
│               │   └── studio-<hash>.css
│               └── manifest.json
└── web-studio/                 # ← 新建前端工程
    ├── package.json
    ├── tsconfig.json
    ├── vite.config.ts
    ├── tailwind.config.ts
    ├── postcss.config.js
    ├── index.html
    ├── src/
    │   ├── main.tsx
    │   ├── App.tsx
    │   ├── api/
    │   │   └── affiliate.ts
    │   ├── store/
    │   │   └── runStore.ts
    │   ├── graph/
    │   │   ├── graphDefinition.ts   # 静态 DAG 拓扑
    │   │   ├── GraphCanvas.tsx
    │   │   ├── nodes/
    │   │   │   ├── StepNode.tsx
    │   │   │   └── DecisionNode.tsx
    │   │   └── useGraphAnimation.ts # 根据 trace 逐步点亮
    │   ├── panels/
    │   │   ├── InputPanel.tsx
    │   │   ├── CopyResultCard.tsx
    │   │   ├── NodeDetailDrawer.tsx
    │   │   ├── RetrievalPanel.tsx
    │   │   ├── ToolsPanel.tsx
    │   │   └── DebugPanel.tsx
    │   ├── components/ui/           # Button/Card/Tabs/Badge/Tooltip/Skeleton
    │   └── styles/
    │       └── index.css            # Tailwind 入口
    └── README.md
```

## 5. UI 设计

### 5.1 布局（三栏）

```
┌────────────────────────────────────────────────────────────────────────┐
│  Header: Affiliate AI Studio            [learn] [gh] [light/dark]      │
├────────────────┬───────────────────────────────────┬───────────────────┤
│  Input Panel   │  Graph Canvas (React Flow)        │  Result Panel     │
│  (fixed 320)   │  (flex, auto-fit)                  │  (380, tabs)      │
│                │                                    │                   │
│  - product_url │   normalize ─▶ fetch ─▶ commission │  ┌─ Copy ───────┐ │
│  - product_text│                            │       │  │ title / hook │ │
│  - platform    │                ┌───────────┼─✗     │  │ body / cta   │ │
│  - locale      │                ▼           ▼       │  │ tags         │ │
│  - style       │            retrieve ─▶ generate    │  └──────────────┘ │
│  - threshold   │                            │       │  Retrieval | ... │
│  - ☑ compress  │                            ▼       │                   │
│  - [▶ Run]     │                        finalize    │                   │
│                │  ◀ (bottom) mini timeline ▶        │                   │
└────────────────┴───────────────────────────────────┴───────────────────┘
```

- **暗色为默认**（契合"AI 工作室"气质），顶部右上角切浅色
- 响应式：< 1280 折叠 Result 为底部抽屉，< 768 Input 折叠为顶部 sheet

### 5.2 DAG 可视化细节

拓扑写死在 `src/graph/graphDefinition.ts`（因为后端 Graph 结构稳定，不值得做动态推导）：

```ts
export const NODES = [
  { id: "normalize_input",        label: "Normalize",       type: "step"     },
  { id: "fetch_product_info",     label: "Fetch Product",   type: "step",    tool: "fetch_product_info" },
  { id: "check_commission",       label: "Check Commission", type: "decision", tool: "check_commission" },
  { id: "retrieve_policy_context", label: "Retrieve KB",     type: "step",    tool: "search_policy_kb" },
  { id: "generate_structured_copy", label: "Generate Copy",  type: "llm" },
  { id: "finalize_rejected",      label: "Reject",          type: "terminal", variant: "error" },
  { id: "finalize_accepted",      label: "Accept",          type: "terminal", variant: "ok" },
];

export const EDGES = [
  ["normalize_input", "fetch_product_info"],
  ["fetch_product_info", "check_commission"],
  ["check_commission", "retrieve_policy_context", { label: "accepted" }],
  ["check_commission", "finalize_rejected",       { label: "rejected" }],
  ["retrieve_policy_context", "generate_structured_copy"],
  ["generate_structured_copy", "finalize_accepted"],
];
```

**节点视觉状态**（由 `useGraphAnimation` 基于 trace 推导）：
- `idle` 灰描边
- `active` 蓝色脉冲边框 + spinner（当前正在播放的那一步）
- `done` 绿色实底 + √
- `skipped` 虚线描边 + 半透明（例如被 reject 跳过的 retrieve/generate 分支）
- `error` 红色描边 + ⚠

**动画**：服务器是**一次性**返回完整 trace 的（不是流式），因此前端 on submit 拿到数据后，用 `setTimeout` 以 ~350ms/step 的节奏逐步推进节点状态，制造"在跑"的感觉。第一版先不做真流式（后端不支持 SSE），在 debrief 里标注为后续改进点。

**节点点击**：打开 `NodeDetailDrawer`（从右侧滑入），展示该节点的：
- 对应 trace 条目（`summary` / `status` / `input_snapshot` / `output_snapshot`）
- 如果该节点调用了 tool，匹配对应 `tools[]` 条目（按 tool 名匹配），展示 `input` / `output`
- 代码跳转提示："对应 `python/graph/affiliate_graph.py` · `normalize_input`"（纯展示，不做真跳转）

### 5.3 Result Panel（右栏 Tabs）

Tab 1 · **Copy**（默认）
- `accepted`：大卡片，title（18px bold）、hook（灰）、body（多行）、cta（高亮按钮样式）、tags（chip）
- `rejected`：红底卡片，`reason` + 佣金率 gauge（0 / threshold / actual 三点对比）
- `fallback=true` 时 title 右侧追加一个 🟡 `Fallback fixture` 徽章，鼠标悬浮 tooltip 提示"未匹配到真实商品，这是兜底数据"

Tab 2 · **Retrieval**
- hits：文档卡片（title / source / score badge / excerpt 高亮命中词）
- compressed：对比视图，`hit.text` vs `compressed.excerpt`，diff 高亮

Tab 3 · **Tools**
- 按时间顺序渲染每个 tool call：icon + 名字 + 折叠展开的 input / output
- 折叠组件用 `<details>` 原生语义

Tab 4 · **Debug**
- `rendered_prompt`（代码块，shiki 或 Prism 高亮）
- `raw_output`（纯文本）
- "Copy raw JSON"按钮（整个响应）

### 5.4 状态处理

| 状态 | 展示 |
|---|---|
| 初始 | 左栏表单预填；中栏 DAG 全灰；右栏空态："填写左边参数，点击 Run 看 Graph 跑起来" |
| 运行中 | Run 按钮变 spinner；DAG 节点逐个点亮动画 |
| 成功（accepted） | Copy tab 渲染；DAG 全绿 |
| 成功（rejected） | Copy tab 红卡；generate/retrieve 节点标 `skipped` |
| tool_error（当 strict=True 或将来其它 tool 失败） | DAG 上对应节点标 `error`；右栏顶部一个红色 banner，显示 `error.code` + `error.message` + 复制按钮（把完整响应塞剪贴板）+ `action` / `session_id` |
| worker_error | 同上，banner 标"Worker 通信错误"，提示去看 Go 进程 stderr（`[worker]` 前缀） |
| 网络/500 | 通用错误卡片 |

## 6. Go 侧改动

### 6.1 `studio.html` 变成 Vite shell

```html
<!doctype html>
<html lang="zh-CN" class="dark">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{{ .Title }}</title>
  {{ if .DevMode }}
    <script type="module" src="http://localhost:5173/@vite/client"></script>
    <script type="module" src="http://localhost:5173/src/main.tsx"></script>
  {{ else }}
    <link rel="stylesheet" href="/static/dist/{{ .CSSFile }}" />
    <script type="module" src="/static/dist/{{ .JSFile }}"></script>
  {{ end }}
</head>
<body>
  <div id="root"></div>
</body>
</html>
```

Go 端 `StudioPage` 读 `web/static/dist/manifest.json` 拿到入口 JS/CSS 文件名，塞给模板：

```go
type studioAssets struct {
    JSFile  string
    CSSFile string
    DevMode bool
}
```

- `AFFILIATE_STUDIO_DEV=1` → `DevMode=true`（本地开发）
- 否则读 manifest；manifest 不存在时返回一个明确的 500 提示"请先跑 make build-web"

### 6.2 新增 Makefile（可选但推荐）

```
make build-web     # cd web-studio && npm ci && npm run build
make dev-web       # cd web-studio && npm run dev
make dev-go        # cd go && AFFILIATE_STUDIO_DEV=1 go run ./cmd/studio
```

### 6.3 .gitignore

加：
```
web-studio/node_modules/
web-studio/dist/
affiliate-ai-studio/go/web/static/dist/
```

### 6.4 handler 测试

`TestStudioPageRenders` 当前只测 "包含 Affiliate AI Studio"字样，需要改成测：
- 提供 stub manifest 时，HTML 里包含 `/static/dist/<hash>.js`
- DevMode 时，HTML 里包含 `localhost:5173`
- manifest 缺失时，返回 500 且 body 有明确提示

## 7. API 契约

**不改动 `POST /api/affiliate/run` 的入出参**。前端 TypeScript 类型从 trace 和 tool call 的已有 JSON 形状推导。

为了让前端拿到 fallback 信号，依赖已经落地的 `product_info.fallback=true` 字段（见上一次对话改动）。

## 8. 构建 & 开发流程

**本地开发**（两个终端）：
```bash
# 终端 A：Go
cd affiliate-ai-studio/go
AFFILIATE_STUDIO_DEV=1 go run ./cmd/studio

# 终端 B：前端
cd affiliate-ai-studio/web-studio
npm install
npm run dev            # http://localhost:5173 自带 /api/* proxy 到 :8080
```
访问 `http://localhost:5173/studio` 走 Vite，HMR 热更新。

**生产构建**：
```bash
cd affiliate-ai-studio/web-studio
npm run build          # 产出到 ../go/web/static/dist/
cd ../go
go run ./cmd/studio    # 直接读 manifest，生产模式
```

## 9. 测试策略

| 层 | 覆盖 |
|---|---|
| 前端单元 | `useGraphAnimation`（trace → node states 转换）、`api/affiliate`（错误分支）用 Vitest |
| 前端视图 | 关键组件（`CopyResultCard`、`NodeDetailDrawer`）用 RTL（React Testing Library）快照 + 交互测试 |
| Go | 更新 `TestStudioPageRenders`：stub manifest、DevMode、missing manifest 三条路径 |
| E2E | 第一版不引入 Playwright；在 README 写出手动验收清单（§11） |

## 10. 分步交付里程碑

尽管用户选"一次做完整版"，我仍建议**内部分 3 个 commit**便于回滚，但一次 PR 交付：

- **M1 脚手架**：web-studio Vite 项目 + Tailwind + 路由通 + 表单 + 调 API + JSON dump（等价于现状，验证前后端集成）
- **M2 DAG + 节点动画 + Drawer**：核心交付物
- **M3 Result 卡片化 + 错误/空态 + 暗色主题打磨 + 测试**

## 11. 手动验收清单

1. `npm run build && go run ./cmd/studio`，访问 `/studio` 能看到新界面
2. 用现有 fixture URL 运行，DAG 逐步点亮，Result 显示 accepted copy 卡片
3. 用一个 **unknown Shopee URL** 运行，Copy 卡片上有 🟡 Fallback 徽章，Retrieval/Tools 都有数据
4. `min_commission_rate=0.5`（强制拒）运行，retrieve/generate 节点标 skipped，Copy 显示 rejected 卡片
5. 临时把 `product_tool` 的 `strict=True`，触发 tool_error，顶部 banner 正确显示 `action=run_affiliate_graph` / session_id / message
6. 切浅色主题，排版不崩
7. < 1280 宽度，Result 收为底部抽屉
8. Vite dev 模式 HMR 改一个组件样式实时生效

## 12. 风险 & 权衡

| 风险 | 应对 |
|---|---|
| 引入 Node 工具链，增加项目复杂度 | README 写清开发/构建流程；保持 Go 一键 `go run` 可用（依赖 manifest 存在） |
| DAG 动画是"假的"（不真流式） | 明确标注；后续若要真流式，改 `/api/affiliate/run` 为 SSE，后端在每个节点后 flush 一条 trace 事件 |
| 引入 Tailwind 与 learn.css 老样式打架 | React app 挂在 `#root`，样式作用域完全独立；learn.html 不受影响 |
| 构建产物进 git？ | 不进。CI 里 build；本地 README 提醒 |

## 13. 待确认项（请你回答）

1. **package manager**：npm / pnpm / yarn？（我默认 npm）
2. **是否接受引入 Tailwind**？（我倾向接受；不接受的话改成手写 CSS modules，但工作量会增加 ~30%）
3. **是否允许我把 React Flow 作为依赖**？（MIT 许可，无额外服务调用）
4. **make 文件你是否用**？（不用的话我就写到 README 里，不建 Makefile）
5. **生产构建产物进 git 吗？** （我强烈建议不进；但如果你希望 clone 即可跑不用 build，会说）

---

## 14. 如果你同意，我下一步会做

按 §10 三个 milestone 执行，期间：
- 每个 milestone 完成后跑对应测试 + `go build`
- 最后一次性 `ReadLints`、手动跑一次 §11 清单中前 4 条，把结果截图回给你
