# ADR-0003 · 推理后端默认 llama.cpp，备选 Ollama

- 状态：Accepted
- 日期：2026-06-13

## 背景

本项目支持两类硬件：MacBook Pro M1 (Metal) 与 RTX 5070 (CUDA, sm_120, Blackwell)。需要选定一个跨平台一致的本地 OpenAI 兼容推理后端。

## 决策

- **默认**：`llama.cpp` 的 `llama-server`。
- **备选 (quickstart)**：`Ollama`。
- **不采用 (此阶段)**：`vLLM` / `TensorRT-LLM` / `TGI`。

## 理由

- **跨平台一致**：llama.cpp 同一份配置思路在 Metal 与 CUDA 都能跑通，OpenAI 兼容度高 (含 `tools` 字段、流式)。
- **构建可控**：CUDA 12.x 上预编译 release 可用；必要时也能本地编译。
- **`--jinja` 支持模型自带 chat template**：Qwen2.5 的 function call 协议正确序列化要靠这个。
- **5070 (sm_120)**：vLLM 对 Blackwell 的支持取决于发布版本与 PyTorch 后端。把 vLLM 留在"未来加分项"，不挡住主路。
- **Ollama 作为备选**：装机一行命令、模型库齐全，作为初学者的快速通道；功能上是 llama.cpp 的子集。

## 反方意见

- llama.cpp 在多并发吞吐上不如 vLLM。 → 学习项目并发量低，吞吐不是瓶颈。
- Ollama 默认只暴露 11434，端口与 llama.cpp 自定义端口冲突需要协调。 → 配置一律走 `OPENAI_BASE_URL`，对调用方无差异。

## 影响

- M0 文档与 M5/M10 的多 server 起服流程都按"llama.cpp 优先 + Ollama 备选"两套写。
- agent 代码层只面向 OpenAI 协议，与具体后端无耦合。
- 未来若 vLLM 在 sm_120 完整可用，可作为 L/XL 档的可选高吞吐替代，不需要改 agent 代码。
