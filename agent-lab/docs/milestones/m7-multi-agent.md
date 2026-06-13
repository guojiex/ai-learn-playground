# M7 · Multi-Agent 协作

**前置**：M6  
**推荐档**：L / XL  
**预计代码量**：~700 行

## 学习目标

- 把"一个 Agent 干所有事"拆成多个角色，理解角色边界与协作协议。
- 设计一个最小可用的消息总线 (Message Bus)，处理路由、终止、循环检测。
- 看清楚多 agent 不是"必胜银弹"——讨论 token 成本与质量的平衡。

## 关键概念

- **角色 (Role)**：每个 sub-agent 有自己的 system prompt + 工具子集 + 输出契约。
- **Message Bus**：协调消息传递，可同步 (轮转) 或异步 (事件)。
- **终止条件**：
  - 轮次上限。
  - Critic 给出 `approve=true`。
  - 关键产物字段齐全 (例如 `title` & `body` & `tags`)。
- **角色互踢回旋 (loop)**：Critic 反复打回 → Writer 反复改 → 最终 token 爆。要在终止条件里防御。

## 角色基线

| 角色 | 主要输入 | 主要输出 | 工具子集 |
|------|----------|----------|----------|
| Researcher | goal, sku_id | facts (规格/卖点/竞品) | `kb_search`, `product_lookup`, `http_get` |
| Writer | facts, persona | draft (title/body/tags) | 无 (纯生成) |
| Critic | draft, platform | issues[] / approve | `platform_lint`, `slang_check` |
| Compliance | draft | violations[] / approve | `platform_lint` (敏感词维度), `sqlite_query` |

## 要写的代码

```
agent-lab/
├── cmd/
│   └── multi/main.go            # 多 agent 演示
├── internal/
│   └── agent/
│       ├── multi.go             # MultiAgent: 协调 + 终止
│       ├── bus.go               # Message Bus (内存版)
│       └── roles/
│           ├── researcher.go
│           ├── writer.go
│           ├── critic.go
│           └── compliance.go
```

`MultiAgent.Run` 大致：

```go
for round := 0; round < maxRounds; round++ {
    facts := researcher.Step(goal)
    draft := writer.Step(facts)
    issues := critic.Step(draft)
    violations := compliance.Step(draft)
    if approve(issues, violations) { return draft }
    feedback = mergeFeedback(issues, violations)
    goal = goal.WithFeedback(feedback)
}
```

## 业务表现

```text
round 1
  researcher: 收集 6 条卖点, 3 条竞品
  writer    : 出稿 (412 tokens)
  critic    : issues=2 (开头不抓人, 标签 4 个偏少)
  compliance: violations=0
  -> 不通过, 反馈给 writer
round 2
  writer    : 重写 (438 tokens)
  critic    : issues=0, approve
  compliance: violations=0
  -> 通过
output: title / body / tags
```

## UI 增量 (M7)

- **Multi 面板**：4 列 (Researcher / Writer / Critic / Compliance)，每列是该角色的消息流；顶部显示当前 round / 终止条件。
- 反馈合并 (`mergeFeedback`) 用横跨四列的虚线连接，强调"Critic+Compliance → Writer"的回环。
- 任一角色被卡住 (例如 Critic 反复打回) 时, UI 头部出现循环防御警告。
- 与 M6 的关系：Plan 面板里的 `agent: writer` 节点，点击后跳到 Multi 面板对应 round。

## 验收标准

- [ ] 4 个角色独立可测，每个都有自己的单测 (fake LLM 验证 prompt 拼装)。
- [ ] Message Bus 能完整持久化一轮的所有消息到 SQLite, 可回放。
- [ ] 显式的循环防御：连续 K 轮无改进 (相似度 > 阈值) 即强制终止。
- [ ] 输出至少 1 条对照报告：单 agent (M6) vs. multi-agent (M7) 的 token 成本与质量。

## 进阶练习

1. 让 Critic 与 Compliance 并发跑，加速一轮 round。
2. 加一个 `Editor` 角色处理"风格统一"，看是否值得多花的 token。
3. 把 MessageBus 替换成异步事件版 (channel + worker pool), 体验竞态问题。

## 衔接

- 下一站 [M8](m8-hitl.md)：在 Critic / Compliance 不通过时把决策权交给人类。
- 或者 [M9](m9-observability-eval.md)：开始系统化评测多 agent 的产出。
