# M10 · 模型路由

**前置**：M2 + 多模型已就绪 (M5 之后, 与 M9 互补)  
**推荐档**：L (XL 加分)  
**预计代码量**：~500 行

## 学习目标

- 起多个本地推理 server (3B / 7B / 14B)，让 agent 根据任务 / 上下文长度 / 失败情况自动选择。
- 学会用"标签 + 规则 + 失败降级"组合做路由，而不是只靠模型名硬编码。
- 用 M9 的评测验证路由策略真的省时延 / 提质量。

## 关键概念

- **模型注册表 (Model Registry)**：每个模型有 `name, base_url, ctx, tags (e.g. "fast","reason"), max_input_tokens, est_tps`。
- **路由规则**：
  - 任务标签：`title -> fast` (3B) / `body -> default` (7B) / `plan -> reason` (14B)。
  - 上下文长度：超出 fast ctx 自动升级。
  - 失败降级：原模型 5xx / parse fail 时切到下一档。
- **A/B 切换**：按比例分流相同任务到两个模型，配合 M9 评测出收益。
- **档位策略**：S 档没有 14B，路由表自动把"reason"映射到本档可用的最强模型 (1.8B)，并打 warning。

## 要写的代码

```
agent-lab/
├── cmd/
│   └── route/main.go            # 演示: 同一任务走不同标签
├── internal/
│   └── llm/
│       ├── router.go            # 路由器
│       ├── registry.go          # 模型注册表 (yaml 加载)
│       └── policy.go            # 规则: 标签 / ctx / fallback
└── config/
    └── models.yaml              # 模型注册表
```

`models.yaml` 示例：

```yaml
profile: L
models:
  - name: qwen2.5-3b-instruct
    base_url: http://127.0.0.1:8082/v1
    ctx: 8192
    tags: [fast, title, tag]
  - name: qwen2.5-7b-instruct
    base_url: http://127.0.0.1:8080/v1
    ctx: 8192
    tags: [default, body, critic]
  - name: qwen2.5-14b-instruct
    base_url: http://127.0.0.1:8083/v1
    ctx: 8192
    tags: [reason, planner]

routes:
  - match: { task: "title" }
    use: fast
    fallback: [default]
  - match: { task: "plan" }
    use: reason
    fallback: [default]
  - match: { ctx_tokens_gt: 6000 }
    use: default
```

## 业务表现

```text
$ go run ./agent-lab/cmd/route --task title  -m "..."
[router] match=task:title -> qwen2.5-3b-instruct (fast)

$ go run ./agent-lab/cmd/route --task plan   -m "..."
[router] match=task:plan  -> qwen2.5-14b-instruct (reason)
[router] ctx=2114 ok

$ go run ./agent-lab/cmd/route --task body --huge-context
[router] match=ctx_tokens_gt:6000 -> qwen2.5-7b-instruct (default)
```

## UI 增量 (M10)

- **Router 面板**：模型注册表表格 (name / base_url / ctx / tags / 状态)。
- 命中柱图：按 task 标签 / 模型聚合的最近 N 次调用。
- 最近 50 次调用列表：起始模型 → fallback chain → 命中模型 → 时延，点击跳到对应 trace。
- A/B 切换条：当前比例 (e.g. 70/30) 可在 UI 调整 (写回 `models.yaml` 或运行时内存)。

## 验收标准

- [ ] 路由器与 LLMClient 解耦：调用方只传 `task` 标签 / `model` 名，路由内部决定打到哪。
- [ ] 失败降级在 trace 里清晰可见 (span.attrs.fallback_chain)。
- [ ] 使用 M9 跑出"路由前后"评测对照，至少给出一组指标差。
- [ ] S 档 (单模型) 与 L 档 (3 模型) 配置都能跑通，无需改代码。

## 进阶练习

1. 把"任务标签"做成可学习：用历史 trace 训练一个轻量分类器决定路由。
2. 加入"成本"维度 (本地以 tps × tokens 折算)，做 cost-aware 路由。
3. 用 5070 同时起 3B + 7B 两份 server，体验显存共存与并发吞吐。

## 衔接

下一站 [M11](m11-capstone.md)：把 M0–M10 的能力组合起来，做出一个完整电商文案 agent。
