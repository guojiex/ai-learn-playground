# M11 · Capstone：综合电商文案 Agent

**前置**：M0–M10 全部  
**推荐档**：L / XL  
**预计代码量**：~600 行 (主要是组合 + 配置)

## 学习目标

- 把前面 10 个里程碑的能力组合成一个**真正可用**的电商文案 agent。
- 跑一份**最终评测报告**，用数据说话："这一套相比基础 agent 提升了什么"。
- 形成一个可以反复迭代的工程基线，后续接新场景只动数据/规则，不动核心代码。

## 系统组合

```
输入: SKU URL / 描述 / 平台 / 风格
        │
        ▼
┌──────────────────────────┐
│  PlannerAgent (M6)       │  --reason 路由 (M10)
└──────────────────────────┘
        │ Plan (DAG)
        ▼
┌──────────────────────────┐
│  Multi-Agent (M7)        │
│   - Researcher           │  --default
│     · kb_search (M5)     │
│     · product_lookup     │
│   - Writer               │  --default
│   - Critic               │  --default
│   - Compliance           │  --fast
│  Memory (M4) 长期口吻库  │
└──────────────────────────┘
        │
        ▼ (高风险动作)
┌──────────────────────────┐
│  HITL (M8)               │
└──────────────────────────┘
        │
        ▼
输出: 多平台文案 + Citations + Trace + 评测报告
```

## 要写的代码

```
agent-lab/
├── cmd/
│   └── capstone/main.go         # 主入口: 一条命令完成上新流程
└── internal/
    └── capstone/
        ├── pipeline.go          # 把前述模块串起来
        ├── persona.go           # 默认电商文案助理人设
        └── outputs.go           # 多平台输出格式化
```

## 业务表现

```text
$ go run ./agent-lab/cmd/capstone \
    --seller A001 \
    --sku-id sku_001 \
    --platforms shopee_tw,xhs_tw \
    --style girlfriend

[planner] 5 tasks
[exec]    researcher → writer → critic → compliance → composer
[hitl]    跳过 (无 High-risk 动作)

--- shopee_tw ---
【日本製】今治本舗 純棉吸水浴巾 70x140 ...

--- xhs_tw ---
#今治毛巾 / 蓬鬆吸水到哭出來 ...

[trace] 18 spans, 12.3s, 4128 in / 1102 out tokens
[eval]  judge_score=4.4/5  slang_hit=78%  platform_lint=ok
report -> docs/reports/capstone_2024-06-12.md
```

## UI 增量 (M11)

- **Capstone 一页**: 顶部参数 (seller / sku / platforms / style)，一键 Run。
- 中部三栏：Plan DAG (M6) / Multi-Agent 消息流 (M7) / Trace 时间线 (M9) 同屏展示, 用统一 trace_id 关联。
- 底部输出区：分平台标签页, 文案 + citation; 旁边小卡片显示 token 成本与评测分数。
- 触发 HITL 时, 顶部弹出待办抽屉; approve 后页面状态自动 resume。
- 演示用: 提供一组 demo SKU 与 seller, 一键 "Replay 上一次成功 run"。

## 验收标准

- [ ] 一条命令跑完整流程，零交互 (无 HITL 触发时)。
- [ ] 输出至少 2 个平台版本，含 citation。
- [ ] 触发一次模拟"高风险动作" (如 `shopee_publish`)，验证 HITL 落地与 resume。
- [ ] 跑完后自动生成 markdown 报告：goal / plan / 关键 step / 评测分数 / token 成本。
- [ ] 用 M9 评测对比"M11 Capstone vs. M2 单 agent baseline"，给出至少 1 个明显差异维度。

## 进阶练习

1. 把 Capstone 改成 HTTP 服务 (`POST /generate`)，做出最小可演示的产品形态。
2. 给 `seller_id` 接入更复杂的口吻学习：从历史成稿中归纳关键词偏好。
3. 加一个"季节性活动"维度 (中秋 / 周年慶)，验证 prompt + RAG 是否能把活动信息融入文案。

## 终点不是终点

到了这里，你已经"从零手写"了一个具备 planner、multi-agent、memory、RAG、HITL、trace、eval、router 的本地 agent 系统。下一步可选方向：

- 框架对比：用 LangGraph / OpenAI Agents SDK 实现等价系统，量化生产力差异。
- 训练联动：用 Capstone 跑出的高质量样本反哺 `lora/`，再把 LoRA 模型作为 specialist 注入 M10 路由。
- 工程化：HTTP/gRPC 服务 + 鉴权 + 多租户 + 速率限制。
