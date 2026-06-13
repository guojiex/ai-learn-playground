# M4 · 记忆 (短期 + 长期)

**前置**：M3  
**推荐档**：L  
**预计代码量**：~400 行 + SQLite migration

## 学习目标

- 把 M1 的短期滑窗升级为"滑窗 + 摘要"双层。
- 引入跨会话的长期记忆 (KV)，让 Agent 记住"卖家 A 偏好闺蜜风、爱用 Emoji"。
- 用 SQLite 做单文件持久层，所有状态可见、可备份、可回放。

## 关键概念

- **短期记忆**：本轮会话的滑窗 + 摘要，活在内存。
- **长期记忆**：跨会话保留，按 namespace 分区，进 SQLite。
- **摘要触发条件**：估算 token 超阈值，或对话轮数 > N。
- **记忆写入策略**：
  - 显式：用户/工具说"记住 X"，agent 写入。
  - 隐式：每 K 轮跑一次"重要点抽取"，把高价值条目入库。
- **记忆检索**：M4 先做精确 KV (按 namespace + key)；近似检索留给 M5。

## 要写的代码

```
agent-lab/
├── internal/
│   ├── memory/
│   │   ├── shortterm.go         # 已存在 (M1)
│   │   ├── summarizer.go        # 触发摘要 + LLM 调用
│   │   └── kv.go                # 长期 KV, 走 store
│   ├── store/
│   │   ├── sqlite.go            # 打开 agent.db, 自动 migrate
│   │   └── migrations.go        # 初始 schema
│   └── tools/
│       ├── memory_get.go        # 工具: 读取长期记忆
│       └── memory_put.go        # 工具: 写入长期记忆
└── data/
    └── agent.db                 # 运行期生成, .gitignore
```

SQLite 初始 schema (会随后续里程碑扩展)：

```sql
CREATE TABLE memory_kv (
  namespace TEXT NOT NULL,
  key       TEXT NOT NULL,
  value     TEXT NOT NULL,        -- JSON
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (namespace, key)
);

CREATE TABLE conversations (
  id        TEXT PRIMARY KEY,
  seller_id TEXT,
  created_at INTEGER NOT NULL,
  ended_at   INTEGER
);

CREATE TABLE conversation_messages (
  conv_id  TEXT NOT NULL,
  idx      INTEGER NOT NULL,
  role     TEXT NOT NULL,
  content  TEXT NOT NULL,         -- JSON 化 Message
  PRIMARY KEY (conv_id, idx)
);
```

## 业务表现

```text
(第一次会话, seller_id=A001)
> 我喜欢闺蜜风, 多用 Emoji, 价格放最后
[memory_put] seller:A001:tone -> {"style":"girlfriend","emoji":"high","price_position":"end"}

(几天后, 第二次会话, 同一个 seller_id)
> 帮我写个新 SKU 文案
(agent 自动 memory_get seller:A001:tone, 风格命中)
```

## UI 增量 (M4)

- 会话从内存切到 SQLite 持久化，UI 刷新页面不丢历史。
- Chat 面板顶部新增 "seller_id" 切换下拉，演示同一 agent 对不同卖家的口吻分支。
- 新增 **Memory 面板** (作为 Settings 子页或独立页)：浏览 KV (按 namespace 折叠树)，支持只读查看，写操作仍由工具调用驱动以避免破坏一致性。
- 摘要触发的 UI 提示带在 M1 基础上加上"压缩前 / 压缩后" token 估算。

## 验收标准

- [ ] `agent.db` 自动初始化，schema migration 幂等。
- [ ] 短期摘要触发后，trace 里能看到"摘要前 vs. 摘要后"的 message 数量。
- [ ] 长期 KV 至少支持 `seller:{id}:tone` / `seller:{id}:keywords` 两个 namespace。
- [ ] 整段 conversation 可被持久化并 100% 还原。
- [ ] 单测覆盖：摘要被调用、KV 读写、并发写入不破坏 schema。

## 进阶练习

1. 摘要质量评测：手工标 5 段对话的"理想摘要"，用 ROUGE / LLM-as-Judge 比对。
2. 加 `memory_forget(namespace, pattern)` 工具，演示"被遗忘权"。
3. 把 KV 里的 `value` 也通过 `gob` / `encoding/binary` 打包，看 SQLite 上做 blob 与 JSON 的差别。

## 衔接

- 下一站 [M5](m5-rag.md)：把"记忆"扩展为"知识 (向量检索)"。
- 也可以并行做 [M6](m6-planner.md) (但 Planner 用到的上下文裁剪依赖 M4 的 summarizer)。
