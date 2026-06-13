# 03 · 术语表

尽量用同一个名字指代同一件事，避免在不同里程碑里换术语。

## 模型 / 推理

- **LLM**：大语言模型。本项目中默认指本地起的 Qwen2.5 系列。
- **Backend**：本地推理服务进程，如 `llama-server` / `ollama serve`。
- **OpenAI 兼容协议**：`POST /v1/chat/completions`、`POST /v1/embeddings` 等接口规范。本项目所有 LLM 调用都走这条协议。
- **GGUF**：llama.cpp 系列推理后端使用的模型权重文件格式。
- **量化 (quantization)**：用低位宽 (Q4 / Q5 / Q8) 表示权重，换取显存与吞吐。`Q4_K_M` 是常用平衡点。
- **上下文窗口 (context window)**：模型一次能看到的最大 token 数。
- **token 预算**：进入模型前主动控制 prompt 长度，给 response 留出空间。

## 控制流

- **Agent**：能感知输入、调用工具、产生输出的循环程序。本项目中表现为 `Agent` 接口的一个实例。
- **Tool / Function**：Agent 可以调用的外部能力。两者在本项目中等价，统一写作 **Tool**。
- **Function Calling**：模型按 schema 主动产出 `tool_calls` 字段的能力。Qwen2.5-Instruct 原生支持。
- **ReAct**：Reasoning + Acting。每一步先输出 Thought，再决定 Action (调用工具) 或 Final，由 Observation 闭环。
- **Plan-and-Solve**：先让模型整体规划 (产出 Plan)，再分步执行；与 ReAct "走一步看一步" 互补。
- **Loop / Step**：Agent 的一次"思考-执行-观察"循环称为一个 Step；多个 Step 组成一次 Run。
- **Termination**：循环结束条件。常见：模型给出最终回复 / 步数上限 / 评审通过 / 用户中止。

## 协作

- **Multi-Agent**：多个具有不同角色 / 工具集 / 提示词的 Agent 协同。本项目用 Researcher / Writer / Critic / Compliance 四角色作为基线。
- **Message Bus**：Agent 之间交换消息的通道，可同步或异步。
- **HITL (Human-in-the-Loop)**：在敏感节点暂停等待人类决策。
- **Approval**：HITL 中的一条待决记录，包含上下文、待执行动作、决议结果。

## 记忆 / 知识

- **Short-Term Memory**：短期记忆，对应当前会话的滑窗。
- **Summarizer**：把超出窗口的旧消息压缩成 summary。
- **Long-Term Memory**：跨会话保留的信息，进 SQLite (KV 或向量)。
- **RAG (Retrieval-Augmented Generation)**：用检索把相关知识塞进 prompt，再交给模型生成。
- **Embedding**：把文本编码成向量，用于相似度检索。
- **Chunking**：把长文档切成可检索的片段。
- **Rerank**：在初步检索后用更精确 (但更慢) 的方法重排，提升 top-k 命中。
- **Citation**：让模型在输出中标注引用来源，可被工具校验。

## 工程

- **Profile (档位)**：S / M / L / XL，决定默认模型与上下文窗口，**不决定代码路径**。
- **Trace**：一次 Run 内所有 LLM 调用、工具调用、agent step 的结构化日志。
- **LLM-as-Judge**：用一个 (通常更强的) 模型给另一个模型的输出打分。
- **Regression Set**：固定输入集合 + 期望表现集合，用于回归评测。
- **Router**：根据任务标签 / 模型名把请求分发到不同后端 / 模型。

## 业务

- **SKU**：商品最小可售单位。本项目中表现为一份 JSON。
- **平台规范 (platform rules)**：蝦皮 / momo / 小红书台湾 等对文案的字数、敏感词、格式要求。
- **黑话 (slang)**：台湾电商语境下的高情境词，如 "免運"、"現貨"、"小資族"。`lora/` 评测中的"黑话命中率"沿用到 M9。
