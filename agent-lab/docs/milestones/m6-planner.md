# M6 · Planner-Executor

**前置**：M3 + M4 (推荐叠加 M5)  
**推荐档**：L / XL  
**预计代码量**：~600 行

## 学习目标

- 把"走一步看一步" (ReAct) 升级为"先整体规划，再分步执行"。
- 学会用一个 LLM 调用产出**结构化 Plan**，并由代码 (而非另一次 LLM) 来执行调度。
- 处理失败重规划：某个子任务失败时，是重试、跳过、还是请求新计划。

## 关键概念

- **Plan**：一个 DAG，节点是子任务 (含工具/输入)，边是依赖关系。
- **Plan-and-Solve**：先规划再执行，与 ReAct 互补。复杂任务用 Planner，单点任务用 ReAct。
- **Replan**：失败次数 / 时间窗口达到阈值时，把"目前进展 + 失败原因"喂回 Planner，让它给新计划。
- **上下文裁剪**：执行长任务时，每个子任务只看自己需要的上下文，避免膨胀。

## 要写的代码

```
agent-lab/
├── cmd/
│   └── plan/main.go             # CLI 入口: 输入目标, 看 Plan, 选择执行
├── internal/
│   └── agent/
│       ├── planner.go           # PlannerAgent: 产出 Plan
│       ├── executor.go          # 按 DAG 调度子任务 (串/并行)
│       └── plan_types.go        # Plan / Task / Edge 数据结构
```

Plan 结构 (作为 Planner 输出的 JSON 协议)：

```json
{
  "goal": "为 sku_001 在小红书台湾发一篇上新文案",
  "tasks": [
    {"id":"t1","name":"调研同品类爆款","tool":"kb_search","args":{...}},
    {"id":"t2","name":"提炼卖点","depends":["t1"],"tool":"product_lookup","args":{...}},
    {"id":"t3","name":"撰写正文","depends":["t2"],"agent":"writer"},
    {"id":"t4","name":"合规检查","depends":["t3"],"tool":"platform_lint","args":{...}},
    {"id":"t5","name":"组合输出","depends":["t3","t4"],"agent":"composer"}
  ]
}
```

## 业务表现

```text
$ go run ./agent-lab/cmd/plan -m "为 sku_001 在小红书台湾发一篇上新文案"
[planner] generated 5 tasks (DAG: t1 -> t2 -> t3 -> t4 -> t5)
[exec] t1 kb_search ... ok
[exec] t2 product_lookup ... ok
[exec] t3 writer ... ok (412 tokens)
[exec] t4 platform_lint ... fail: emoji 数 > 上限
[replan] reason=t4 failed (emoji 上限) -> 修订 t3 prompt
[exec] t3 writer ... ok
[exec] t4 platform_lint ... ok
[exec] t5 composer ... ok

--- 输出 ---
#今治毛巾 ... 标签5个 ...
```

## UI 增量 (M6)

- **Plan 面板**：左侧 goal 输入 + Generate Plan 按钮；右侧 DAG 可视化 (列布局 + 连线)。
- 任务节点状态用颜色: pending(灰) / running(蓝, 转圈) / ok(绿) / fail(红) / replan(橙)。
- 节点点击展开 `tool/agent/args/输出/耗时`；当前在跑的节点附 SSE 实时刷新。
- 顶部状态条：当前 step / 总 step / 累计 token / replan 次数。
- 失败重规划事件在面板右上角时间线显示。

## 验收标准

- [ ] Planner 输出严格 JSON，解析失败可兜底重新提问。
- [ ] Executor 支持串行 + 受限并发 (依赖图允许并行的节点并发跑)。
- [ ] 子任务失败时，Replan 至多 N 次，超过则报最终失败。
- [ ] 整次 Run 能 dump 一份"plan + 实际执行轨迹"的 JSON, 便于 M9 trace。

## 进阶练习

1. 给 Planner 加 `budget` 字段 (token / 步数), 执行器据此做硬性中止。
2. 比较"Plan 一次性给全 vs. 增量给"的差异 (类似 LATS / Tree-of-Thoughts 的简化)。
3. 用 XL 档跑一遍 Planner，比较 7B vs. 14B 在 plan 合理性上的差距。

## 衔接

- 下一站 [M7](m7-multi-agent.md)：把 Plan 中的 `agent: writer` / `agent: composer` 真正落成多 agent。
- 也可以并行做 [M8](m8-hitl.md)：在 Plan 的关键节点引入审批。
