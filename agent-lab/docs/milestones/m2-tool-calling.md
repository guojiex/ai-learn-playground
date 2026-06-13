# M2 · Tool Calling (原生 function call)

**前置**：M1  
**推荐档**：L (S 上做"会失败"的对照实验)  
**预计代码量**：~400 行

## 学习目标

- 用 Qwen2.5-Instruct 原生支持的 function calling 能力，跑通"模型主动调用工具 → 拿结果 → 再说话"的回环。
- 写出可复用的 `Tool` 接口与 `Registry`，后续所有里程碑都用它。
- 用 JSON Schema 描述工具入参，并在客户端做严格校验。

## 关键概念

- **Tool schema**：`{name, description, parameters: JSONSchema}`。`description` 必须**面向模型**，不是给人看的。
- **OpenAI 协议下的 tool 流转**：
  1. assistant 返回 `tool_calls: [{id, type:"function", function:{name, arguments}}]`。
  2. 客户端按 `name` 找到 Tool，解析 `arguments` 调用，得到字符串结果。
  3. 用 `role=tool, tool_call_id=<id>, content=<result>` 把结果回填到 messages，再发一次 chat。
  4. 直到 assistant 的 `finish_reason=stop`。
- **并行调用**：assistant 可能一次性返回多个 `tool_calls`；要并发执行并按 id 回填。
- **失败处理**：工具报错时**把错误回填给模型**，而不是 panic；模型常常能换工具或换参数重试。

## 要写的代码

```
agent-lab/
├── cmd/
│   └── agent/main.go            # 替代 chat: 带工具的多轮 agent
├── internal/
│   ├── tools/
│   │   ├── tool.go              # Tool 接口 + Registry + JSON Schema 帮助函数
│   │   ├── product_lookup.go    # 读 data/products/*.json
│   │   ├── price_format.go      # NT$690 (限時免運) 这种串
│   │   ├── platform_lint.go     # 字数 / 敏感词 / 标签数 校验
│   │   └── slang_check.go       # 黑话密度
│   └── agent/
│       └── tooling.go           # 工具调用循环 (parallel + 回填 + 上限)
```

`Tool` 接口固定：

```go
type Tool interface {
    Schema() ToolSchema
    Invoke(ctx context.Context, args json.RawMessage) (string, error)
}
```

## 业务表现

```text
> 帮我为 sku_001 在蝦皮写一段标题, 要带"現貨""免運"
[tool] product_lookup({"id":"sku_001"}) -> {...今治毛巾...}
[tool] price_format({"price_twd":690,"shipping":"現貨 / 24h 出貨"}) -> "NT$690 · 現貨 · 限時免運"
[tool] platform_lint({"platform":"shopee_tw","text":"..."}) -> {"ok":true,"len":54}
【日本製】今治本舗 純棉吸水浴巾 70x140 蓬鬆不掉毛 現貨免運 NT$690
```

S 档 (1.8B) 应该会观察到**工具调用经常 schema 错位 / 不调工具直接幻觉**，这是有意保留的对照。

## UI 增量 (M2)

- 消息流中遇到 `tool_calls`：插入折叠卡片，显示 `tool name / args (JSON) / 调用耗时 / 结果摘要`，点击展开看完整 JSON。
- 多 `tool_calls` 并行执行时，UI 用同一组卡片并显示状态徽标 (running / ok / fail)。
- 右侧新增 **Tools 面板** (`/tools`)：列出当前注册的工具及其 JSON Schema；最近调用记录列表 (最近 50 条)。
- 失败回填给模型的 error，UI 在卡片底部用红色摘要展示。

## 验收标准

- [ ] `Registry` 能批量注册/检索工具，`Schemas()` 返回的 JSON Schema 直接喂给 LLM。
- [ ] agent 循环最多 N 步 (默认 8)，超过则报错并打 trace。
- [ ] 工具报错时错误信息以 `role=tool` 回填，模型有机会重试。
- [ ] 多个 `tool_calls` 并发执行 (`errgroup` 或类似)。
- [ ] 单测：用 fake LLM (固定吐 `tool_calls`) 验证回环逻辑，不依赖真模型。

## 进阶练习

1. 给 `Tool` 加 `DryRun` 选项，用于 HITL (M8) 的"预览即将执行的副作用"。
2. 在 `platform_lint` 里把违规位置以 `range` 数组返回，让模型可以精准重写。
3. 加一个 `python_eval` 工具 (sandbox: 子进程 + 超时 + cpu/mem 限制)，体验"代码解释器"型 Agent。

## 衔接

- 下一站 [M3](m3-react.md)：脱离原生 function call，亲手写 ReAct 协议。
- 也可以直接跳 [M4](m4-memory.md)：在 M2 的 agent 上加记忆。
