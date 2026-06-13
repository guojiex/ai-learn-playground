# ADR-0004 · 默认模型 Qwen2.5-7B-Instruct (L 档)

- 状态：Accepted
- 日期：2026-06-13

## 背景

需要为"主力档 (L)"选定一个默认 chat 模型，作为大多数 milestone 的基线。候选包括 Qwen2.5、Llama-3.x、Gemma 2、Yi 1.5 等。

## 决策

- **L 档默认**：`Qwen2.5-7B-Instruct` (GGUF Q4_K_M)。
- **S 档**：`Qwen1.5-1.8B-Chat` (与 `lora/` 基模一致)。
- **M 档**：`Qwen2.5-3B-Instruct`。
- **XL 档**：`Qwen2.5-14B-Instruct`。
- **Embedding**：`bge-m3` (优先) 或 `bge-small-zh-v1.5` (低显存备选)。

## 理由

- **中文 + 繁中**：Qwen 系列中文表现稳定，包含台湾繁中语料，与本项目的电商场景天然匹配。
- **原生 function calling**：Qwen2.5-Instruct 自带 tool 协议 (`--jinja` 启用模型 template 后)，M2 落地省一层适配。
- **同系列多档位**：S/M/L/XL 全部用 Qwen 系列，行为一致，便于 M10 路由学习与对照。
- **Q4_K_M 量化**：7B 占约 5–6GB，5070 (12GB) 还有显存留给 KV cache 与 embedding server 共存。
- **保留 Qwen1.5-1.8B 作为 S 档**：与 `lora/` 已用基模一致，不增加额外依赖；同时它在 M2 (原生 function call) 上的弱表现是有意义的对照样本。

## 反方意见

- Llama-3-8B 在英语任务上更强。 → 本项目以中文电商为主，Qwen 收益更高。
- Qwen2.5-Coder 在代码工具调用上更稳。 → 本项目工具调用以业务工具为主，不需要专项 Coder 模型；如未来要做 `python_eval` 类工具可在 M2 进阶练习中切到 Coder。

## 影响

- M0/M1 默认 prompt 与 stop 序列按 Qwen2.5 chat template 调优。
- M2 默认走 Qwen2.5 原生 tool 协议，S 档上做"工具协议失败率"对照实验。
- M5 默认走 bge-m3；显存吃紧时换 bge-small-zh-v1.5，由 `AGENTLAB_MODEL_EMBED` 控制。
- 未来切换到 Qwen3 / Qwen2.5-VL 等新版本时，仅需改 ADR + 默认值，代码不动。
