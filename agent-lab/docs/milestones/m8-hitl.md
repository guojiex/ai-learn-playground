# M8 · Human-in-the-Loop

**前置**：M6 (推荐 M7)  
**推荐档**：L  
**预计代码量**：~400 行

## 学习目标

- 在 agent 关键节点暂停，把决策权交给人；同时保证状态可持久化、可恢复。
- 设计 approval 数据结构，让"待审批"成为一种可以被工具/CLI/Web UI 操作的对象。
- 学会区分"可逆动作 (build draft)" 与"不可逆动作 (publish / 改库存)"，只对后者强制 HITL。

## 关键概念

- **Interrupt point**：agent 主动 `return` 一个 `Pending` 状态而非继续执行；此时上下文与下一步动作都已落库。
- **Approval**：一条记录，含 `id, conv_id, action, args, payload, status (pending/approved/rejected/edited), reviewer, reviewed_at`。
- **Resume**：CLI 或脚本审批后，agent 从落库状态继续运行，无需重新跑前面的步骤。
- **风险分级**：基于工具元数据 (`tool.RiskLevel`) 决定是否插入审批，而不是硬编码。

## 要写的代码

```
agent-lab/
├── cmd/
│   └── hitl/main.go             # CLI: 列待审 / 详情 / approve / reject / edit
├── internal/
│   ├── hitl/
│   │   ├── approval.go          # Approval 类型 + 状态机
│   │   └── store.go             # SQLite 持久化
│   ├── tools/
│   │   └── tool.go              # 增加 RiskLevel 字段
│   └── agent/
│       └── interrupt.go         # InterruptError + Resume 支持
```

SQLite schema 增量：

```sql
CREATE TABLE approvals (
  id          TEXT PRIMARY KEY,
  conv_id     TEXT NOT NULL,
  step_idx    INTEGER NOT NULL,
  tool        TEXT NOT NULL,
  args        TEXT NOT NULL,        -- JSON
  payload     TEXT,                  -- 待执行动作的 dry-run 结果
  status      TEXT NOT NULL,         -- pending/approved/rejected/edited
  reviewer    TEXT,
  reviewed_at INTEGER,
  created_at  INTEGER NOT NULL
);
```

## 业务表现

```text
$ go run ./agent-lab/cmd/multi -m "为 sku_001 上架到蝦皮"
...
[hitl] pending approval: ap_20240612_001
        tool: shopee_publish (RiskLevel=High)
        args: {"sku_id":"sku_001","title":"...","price":690}
        payload preview: 即将创建商品页, 库存=32

$ go run ./agent-lab/cmd/hitl list
ap_20240612_001  pending  shopee_publish  conv_xxx

$ go run ./agent-lab/cmd/hitl approve ap_20240612_001 --note "ok"

$ go run ./agent-lab/cmd/multi --resume conv_xxx
[exec] shopee_publish ... ok
```

## UI 增量 (M8)

- **Approvals 面板**：待办列表 (按优先级/创建时间)，状态徽标 (pending/approved/rejected/edited)。
- 详情面板：tool / args / payload preview / risk level；提供 approve / reject / edit 三按钮。
- edit 进入参数编辑 (JSON form)，保存即按新参数 resume。
- 全局徽标：当存在 pending approval 时，左侧导航 Approvals 显示红点计数；任意页面顶部显示一条粘性提示带，方便从 Plan/Multi 面板跳来。
- 与 Trace 面板联动：approval 详情点击 "View trace" 跳到 M9 trace。

## 验收标准

- [ ] `tool.RiskLevel` 至少三档 (Low / Medium / High)，agent 默认对 High 强制 HITL。
- [ ] 中断后 agent 进程退出，重新启动 + `--resume` 能从断点继续。
- [ ] 编辑 (edit) 审批：reviewer 可改 `args`，agent 用改后的参数继续。
- [ ] 拒绝 (reject) 审批：agent 把拒绝原因回填给上一步的角色 (例如 Writer) 重做。
- [ ] 审批记录全程可查，`hitl list --since 24h` 类似命令可用。

## 进阶练习

1. 写一个最简 HTTP 端点 (`/approvals`) 做 web UI 替代 CLI。
2. 把 RiskLevel 从静态改为运行时计算 (基于参数, 例如 price > 阈值就升级)。
3. 加超时策略：pending 超过 N 小时自动 reject 并通知。

## 衔接

- 下一站 [M9](m9-observability-eval.md)：把 trace + 评测落地，HITL 数据是评测样本的天然来源。
- 也可以直接跳 [M11](m11-capstone.md)，因为 HITL 已是工程化的最后一块大砖。
