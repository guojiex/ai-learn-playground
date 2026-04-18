# affiliate-ai-studio-web

Studio v2 前端：React 18 + TypeScript + Vite + Tailwind + React Flow（`@xyflow/react`）。

## 开发模式（HMR）

两个终端同时开：

```bash
# 终端 A：Go 后端，告诉它"前端在 Vite dev server"
cd ../go
AFFILIATE_STUDIO_DEV=1 go run ./cmd/studio
# 监听 :8080

# 终端 B：前端
cd web-studio
npm install        # 仅首次
npm run dev        # 监听 :5173，带 HMR
```

开发时访问 **http://localhost:5173/studio**（Vite 已配置把 `/api/*`、`/learn`、`/static` 代理到 :8080）。

> `:8080/studio` 在 DevMode 下也能用，模板会把 `<script>` 指向 `http://localhost:5173`。

## 生产构建

```bash
cd web-studio
npm run build       # tsc -b && vite build
# 产物输出到 ../go/web/static/dist/（包含 .vite/manifest.json）
```

构建完成后 Go 进程只需要 `go run ./cmd/studio` 即可读取 manifest 渲染 studio.html。

## 测试

```bash
npm test                 # vitest run 一次
npm run test:watch       # 交互式
npm run lint:types       # tsc --noEmit
```

## 目录

- `src/api/` — 后端 HTTP 类型与调用封装
- `src/store/` — zustand 全局 store（表单 + 运行态，localStorage 持久化）
- `src/graph/` — LangGraph DAG 可视化（React Flow 自定义节点 + 动画 hook）
- `src/panels/` — 左输入 / 右结果 / 节点详情抽屉
- `src/components/ui/` — 基础 UI 原语（Button/Card/Tabs/Badge/JsonBlock 等）
- `src/hooks/useTheme.ts` — 暗色/浅色主题切换
- `src/styles/index.css` — Tailwind 入口 + CSS 变量主题

## 环境变量

- `AFFILIATE_GO_URL`（默认 `http://localhost:8080`）：Vite dev server 用它作为 API 代理目标，跨机调试时可改。
