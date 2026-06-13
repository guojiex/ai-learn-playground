# M1 · 多轮对话 + Prompt 工程

**前置**：M0  
**推荐档**：S / L  
**预计代码量**：~250 行

## 学习目标

- 把单次调用扩展成可持续的多轮对话，引入会话状态管理。
- 用 system prompt 把模型"扮演"成一个具体角色：电商文案助理。
- 学会在客户端做 token 预算与上下文裁剪，知道"超窗口"是怎么回事。

## 关键概念

- **角色**：`system` / `user` / `assistant`。一段会话的"人设"放在 system，且只放一份。
- **token 估算**：本地无法精确数 token，用"4 字符 ≈ 1 token (中文偏紧, 1.5 字符 ≈ 1 token)"做粗估即可。
- **上下文裁剪策略**：
  - 简单滑窗：只保留最近 N 轮。
  - 摘要式：超出预算时调 LLM 把最旧若干轮压成 1 段 summary，挂在 system 之后。
- **生成参数**：`temperature` (0.2 严肃 / 0.7 创意)、`top_p`、`stop`、`max_tokens`。

## 要写的代码

```
agent-lab/
├── cmd/
│   └── chat/main.go             # 改造: 进入 REPL, 多轮交互
├── internal/
│   ├── prompt/
│   │   ├── persona.go           # 电商文案助理角色卡
│   │   └── templates.go         # 模板化的 user 提问构造
│   └── memory/
│       └── shortterm.go         # 环形 buffer + 简单 token 估算 + 滑窗裁剪
```

`shortterm.ShortTerm` 接口建议：

```go
type ShortTerm struct {
    sys     llm.Message
    msgs    []llm.Message
    budget  int            // 总 token 预算
    reserve int            // 留给 response 的 token
}
func (m *ShortTerm) Append(msg llm.Message)
func (m *ShortTerm) Snapshot() []llm.Message  // sys + 裁剪后的 msgs
```

## 业务表现

```text
$ go run ./agent-lab/cmd/chat
[L · qwen2.5-7b-instruct]  你好, 我是電商文案助理, 想為哪個商品寫文案? 可以給我商品名/規格/賣點。
> 商品是今治毛巾, 70x140
理解, 還能告訴我品牌跟價位嗎?
> 今治本舗, NT$690
收到。希望文案發在哪個平台?
...
```

- 角色卡决定 agent 一开口就在引导用户提供 SKU 信息。
- 当对话超出预算，会自动摘要并打印一行调试信息：`[summary] compressed 6 turns -> 312 chars`。

## UI 增量 (M1)

- 左侧 Chat 面板增加**会话列表**：新建 / 切换 / 重命名 / 删除；与 server 端 conversation 对应。
- 顶部加可折叠**角色卡 (system prompt)** 编辑区，保存后下条消息生效；当前消息流不重写。
- 当 server 端触发摘要，UI 在消息流内插入一条灰色提示：`摘要: 压缩 N 轮 -> M tokens`。
- 新增 `:reset/:save/:load` 等价的三个按钮 (Reset / Export / Import)。

## 验收标准

- [ ] REPL 支持 `:reset` / `:save <file>` / `:load <file>` 命令。
- [ ] 角色卡能从外部文件加载 (`prompt/persona.tw.md` 等)，便于换风格。
- [ ] 上下文超出预算时会自动裁剪或摘要，不会直接 4xx。
- [ ] 会话能完整保存到本地文件并 100% 还原。

## 进阶练习

1. 给摘要功能加"按重要度而非时间"的策略：用 LLM 标记哪些消息能丢。
2. 把 `Snapshot()` 在每次请求前打 trace，方便看上下文裁剪后的真实形状。
3. 用 `:eval` 命令快速跑一组预设输入，看不同 `temperature` 的差异。

## 衔接

M2 与 M3 是两条并列分支，建议都做：
- [M2](m2-tool-calling.md) 走原生 function calling，最贴近主流框架做法。
- [M3](m3-react.md) 自己写 ReAct 协议，理解原生 function call 在背后做了什么。
