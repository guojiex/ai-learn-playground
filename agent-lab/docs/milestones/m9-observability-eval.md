# M9 · 可观测性 + 评测

**前置**：贯穿全程，正式落地在 M5 之后  
**推荐档**：L  
**预计代码量**：~600 行

## 学习目标

- 给 agent 加一套统一的 trace：所有 LLM 调用、工具调用、agent step 全部可结构化查询。
- 建立一套 agent 级评测：固定输入集合 + LLM-as-Judge + 业务度量 (黑话密度 / 平台合规)。
- 让"改了 prompt 之后比之前更好"这件事**有数据支撑**，而不是凭感觉。

## 关键概念

- **Trace / Span**：一次 Run 是一个 trace，每个 LLM/工具调用是一个 span。span 里至少有 `start, end, name, kind, input, output, tokens, error`。
- **本地"成本"度量**：本地推理没有 API 价格，按 `output_tokens / tps` 估时延，按 `prompt_tokens + output_tokens` 看上下文消耗。
- **LLM-as-Judge**：用一段 rubric (打分细则) 让 LLM 给输出 1–5 分；同模型自评偏松，必要时用更大档 (XL) 做 judge。
- **回归集 (regression set)**：固定 prompts + 期望维度。每次改动跑一遍，盯均值与方差。

## 要写的代码

```
agent-lab/
├── cmd/
│   ├── eval/main.go             # 跑评测集, 出 markdown 报告
│   └── trace/main.go            # 查询 / 导出 trace
├── internal/
│   ├── trace/
│   │   ├── trace.go             # Trace / Span / Recorder
│   │   └── store.go             # 落 SQLite
│   ├── eval/
│   │   ├── runner.go            # 跑 agent + 收集 metrics
│   │   ├── judge.go             # LLM-as-Judge
│   │   └── metrics.go           # 业务度量 (黑话命中率等)
└── data/
    └── eval/
        ├── prompts.jsonl
        ├── judge_rubric.md
        └── expected.jsonl       # 可选
```

SQLite schema 增量：

```sql
CREATE TABLE traces (
  trace_id   TEXT PRIMARY KEY,
  conv_id    TEXT,
  goal       TEXT,
  started_at INTEGER NOT NULL,
  ended_at   INTEGER,
  status     TEXT
);
CREATE TABLE spans (
  span_id    TEXT PRIMARY KEY,
  trace_id   TEXT NOT NULL,
  parent_id  TEXT,
  kind       TEXT NOT NULL,        -- llm / tool / step / agent
  name       TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at   INTEGER,
  attrs      TEXT,                  -- JSON
  input      TEXT,                  -- JSON
  output     TEXT,                  -- JSON
  tokens_in  INTEGER,
  tokens_out INTEGER,
  error      TEXT
);
```

## 业务表现

```text
$ go run ./agent-lab/cmd/eval --suite ecom-v1 --tag baseline
running 18 cases ...
ok 18, fail 0
report -> docs/reports/eval_2024-06-12_baseline.md

$ go run ./agent-lab/cmd/eval --suite ecom-v1 --tag react-tuned
report -> docs/reports/eval_2024-06-12_react-tuned.md

$ diff (mean): judge_score +0.4, slang_hit +6.2pp, tokens -8%
```

## UI 增量 (M9)

- **Traces 面板**：trace 列表 (按 goal / status / 时长 / 命中率过滤)。
- 单 trace 视图：火焰图风格时间线 (LLM / Tool / Step / Agent 用不同色)，悬停显示 attrs。
- Span 详情抽屉：input/output JSON 折叠展示，tokens_in/out、错误信息。
- 评测视图：跑完一次 `eval` 后 UI 出现新报告条目，对照两次报告显示均值 / 方差 / 显著性差。
- 任意一条 trace 与会话/审批互相跳转。

## 验收标准

- [ ] 任意 milestone 的 agent 都能加几行代码就完整接入 trace (用 context.Context 透传)。
- [ ] `eval` 报告同时包含：业务度量 (黑话密度 / 平台合规) + LLM-as-Judge 分数 + token / 时延。
- [ ] 报告可二次运行 (相同 commit + 数据 -> 几乎相同结果)，差异 < 阈值视为稳定。
- [ ] `cmd/trace` 支持按 trace_id / conv_id / 时间范围 / span.kind 查询。

## 进阶练习

1. 把 trace 同步导出成 OTLP，看主流可视化 (Jaeger / Grafana Tempo) 上的呈现。
2. 实现**对照评测**：给同一组 prompts 跑 prompt v1 vs. v2，统计成对差值的显著性。
3. 用 XL 档当 Judge，验证"小模型自评 vs. 大模型他评"的相关性。

## 衔接

- 下一站 [M10](m10-model-routing.md)：评测打底之后再上路由，能做到"路由收益"可量化。
- 或者 [M11](m11-capstone.md)：用 M9 直接验收 Capstone。
