# M3 · 手写 ReAct Loop

**前置**：M1 (与 M2 并列, 互为对照)  
**推荐档**：S / L (S 档主路径)  
**预计代码量**：~500 行

## 学习目标

- 不依赖原生 function calling，自己定义一份 JSON 协议，亲手驱动 Thought-Action-Observation 循环。
- 理解 ReAct 与原生 function call 在能力、稳定性、提示长度上的差异。
- 学会**解析失败兜底**：模型胡乱输出时怎么"硬纠错"。

## 关键概念

- **ReAct 协议**：每一轮 assistant 必须输出形如

  ```json
  {"thought":"...", "action":{"name":"product_lookup","args":{...}}}
  ```

  或

  ```json
  {"thought":"...", "final":"..."}
  ```

  其它格式视为 invalid。
- **Stop 序列**：用 `"\n```"` 这类 stop 防止模型话痨。
- **解析失败兜底**：
  - 第一次失败：把"你的输出格式不对，必须 JSON"作为 system 追加，重发。
  - 第二次失败：自动降级为"final"并把原文作为最终回复，避免死循环。
- **循环上限**：`max_steps`，默认 8；超出即终止并打 trace。

## 要写的代码

```
agent-lab/
├── cmd/
│   └── agent/main.go            # 加 --mode=react|native 切换
├── internal/
│   └── agent/
│       ├── agent.go             # Agent 接口 + Step / Run 类型
│       ├── react.go             # ReActAgent 实现
│       └── parse.go             # JSON 提取 + 容错 (允许 ```json fences)
```

`ReActAgent.Run` 伪代码：

```go
for step := 0; step < maxSteps; step++ {
    resp := llm.Chat(systemReact, history)
    parsed, err := parse(resp.Content)
    if err != nil { /* 兜底重试 */ }
    if parsed.Final != "" { return parsed.Final }
    result, err := registry.Invoke(parsed.Action.Name, parsed.Action.Args)
    history = append(history, observation(result, err))
}
return "", ErrMaxSteps
```

## 业务表现

同样的 SKU 文案任务，在 ReAct 模式下输出会有可读的 Thought 轨迹：

```text
thought: 我需要先拿到 sku_001 的规格
action: product_lookup({"id":"sku_001"})
obs: {...}
thought: 已经有规格, 把价格格式化
action: price_format(...)
obs: NT$690 · 現貨 · 限時免運
thought: 整理标题, 检查长度
action: platform_lint({...})
obs: {"ok":true,"len":54}
final: 【日本製】今治本舗 純棉吸水浴巾 70x140 ...
```

在 S 档 (1.8B) 上做与 M2 的对照：ReAct 通常比原生 function call 表现**更稳**，因为协议简单、提示明确。

## UI 增量 (M3)

- Chat 面板加 mode 切换：`native` / `react`，对应同一组工具的两种调用形态。
- ReAct 模式下，每个 step 渲染为一个有色块：`thought / action / observation`，可整体折叠。
- 解析失败兜底事件 (重试或降级 final) 在 UI 用黄色提示带显示。

## 验收标准

- [ ] 同一组 prompt 与工具集，`--mode=react` 与 `--mode=native` 都能跑通。
- [ ] 模型乱输出时不会无限重试；超过兜底次数即降级为 final。
- [ ] `Step` 结构能完整 dump 到 JSON, 后续 trace 直接复用。
- [ ] 单测覆盖 parse 容错 (代码块包裹 / 多余前后缀 / 单引号 JSON)。

## 进阶练习

1. 实现 `ReflectAgent`：每 K 步追加一段"自我反思"，再继续。比较与基础 ReAct 的差异。
2. 把 stop 序列做成可配置；探索"无 stop"时 1.8B 模型的行为变化。
3. 给协议加 `note` 字段 (持久笔记)，验证它在长任务中是否帮助连续性。

## 衔接

- 下一站 [M4](m4-memory.md)：跨会话记忆。
- 或者直接 [M5](m5-rag.md)：先打通检索增强。
