# 04 · 本地模型栈

本项目**不依赖任何云端 API**。所有 LLM 调用走本地 OpenAI 兼容 server。

## 硬件分档

| 档位 | 硬件 | 主模型 | 量化 | 显存 / 内存占用 | 用途 |
|------|------|--------|------|----------------|------|
| **S (轻档)** | MBP M1 16 GB (统一内存, Metal) | Qwen1.5-1.8B-Chat | GGUF Q4_K_M | ≈ 1.5 GB | M0–M3 全跑通；M4+ 仅做小用例 |
| **M (中档)** | MBP M1 16 GB | Qwen2.5-3B-Instruct | GGUF Q4_K_M | ≈ 2.5 GB | M1+ 跑 tool-calling / RAG 的最低稳定档 |
| **L (主力)** | RTX 5070 12 GB + 32 GB | Qwen2.5-7B-Instruct | GGUF Q4_K_M | ≈ 5–6 GB VRAM | 默认主模型，所有里程碑完整跑通 |
| **XL (进阶)** | RTX 5070 12 GB | Qwen2.5-14B-Instruct | GGUF Q4_K_M | ≈ 9–10 GB VRAM | M6/M7/M11 复杂规划 / 多 agent；显存吃紧时降回 L |
| **Embed** | 任意 | bge-m3 (或 bge-small-zh-v1.5) | f16 / Q8 | < 1 GB | M5 起的检索 embedding |

### 选型理由

- **Qwen2.5 系列**：中文 (含繁中) 表现稳定，且 Qwen2.5-Instruct **原生支持 OpenAI function calling 协议**，省掉一层适配。
- **S 档保留 Qwen1.5-1.8B-Chat**：与 `lora/` 基模一致，环境与 tokenizer 已熟，少装一份。它在 M2 (原生 function call) 的成功率会明显低于 7B，正好用于"小模型为什么撑不住 tool-use"的对照实验。
- **5070 (Blackwell, sm_120)**：CUDA 12.x。**优先用 llama.cpp 的 CUDA 后端**；vLLM 对 sm_120 的支持视发布版本而定，先 llama.cpp 兜底。

### 上下文窗口建议

| 档位 | 默认 `n_ctx` | 备注 |
|------|-------------|------|
| S | 4096 | 多轮对话够，RAG 偏紧 |
| M | 8192 | RAG 与 short summary 可用 |
| L | 8192–16384 | 默认 8192；显存富裕可拉到 16k |
| XL | 8192 | 14B 把 ctx 拉到 16k 显存可能爆，保持 8k |

## 后端方案 (二选一)

两套后端**接入协议完全相同**，agent 代码层零差异。

### 方案 A：llama.cpp + `llama-server` (推荐为默认)

优点：跨平台一致 (Metal / CUDA)，OpenAI 兼容度高，构建可控。

**macOS (M1, S/M 档)**：

```bash
brew install llama.cpp
# 下载 GGUF (示例: Qwen2.5-3B-Instruct-Q4_K_M.gguf)
# 起服:
llama-server \
  -m /path/to/Qwen2.5-3B-Instruct-Q4_K_M.gguf \
  -c 8192 \
  --port 8080 \
  --host 127.0.0.1 \
  --jinja                 # 启用 Jinja 模板, 让 Qwen 的 tool 协议生效
```

**Windows (RTX 5070, L/XL 档)**：

```powershell
# 从 release 下载 llama.cpp CUDA 构建包 (cuda-12.x), 解压到 C:\tools\llama.cpp
C:\tools\llama.cpp\llama-server.exe `
  -m "D:\models\Qwen2.5-7B-Instruct-Q4_K_M.gguf" `
  -c 8192 `
  -ngl 99 `
  --port 8080 `
  --host 127.0.0.1 `
  --jinja
```

- `-ngl 99` 把所有层都压进 GPU；显存吃紧时调小。
- `--jinja` 让 chat template 走模型自带的 Jinja，function calling 才能正确序列化。

**Embedding server (单独一个端口)**：

```bash
llama-server \
  -m /path/to/bge-m3-Q8_0.gguf \
  --embedding \
  -c 2048 \
  --port 8081 \
  --host 127.0.0.1
```

### 方案 B：Ollama (quickstart 备选)

优点：装机一行命令，模型库齐全。

```bash
# macOS / Linux
brew install ollama   # 或 curl -fsSL https://ollama.com/install.sh | sh
ollama serve          # 默认 11434
ollama pull qwen2.5:7b-instruct
ollama pull bge-m3
```

OpenAI 兼容入口：`http://127.0.0.1:11434/v1`。

## agent-lab 的接入约定

不论选哪个后端，统一通过环境变量接入：

```bash
# Chat 主模型
export OPENAI_BASE_URL="http://127.0.0.1:8080/v1"
export OPENAI_API_KEY="sk-local"           # llama.cpp 不校验, 占位即可
export AGENTLAB_PROFILE="L"
export AGENTLAB_MODEL_CHAT="qwen2.5-7b-instruct"

# Embedding (M5 起)
export AGENTLAB_EMBED_BASE_URL="http://127.0.0.1:8081/v1"
export AGENTLAB_MODEL_EMBED="bge-m3"
```

`AGENTLAB_PROFILE` 决定一组**默认值**；其它 `AGENTLAB_MODEL_*` / 上下文窗口可逐项覆写。详见 [02-architecture.md](02-architecture.md) 配置示例。

## 模型获取

本仓库**不内置模型权重**。建议路径：

- macOS：`~/models/`
- Windows：`D:\models\`

权威来源：

- HuggingFace: `Qwen/Qwen2.5-7B-Instruct-GGUF`、`Qwen/Qwen1.5-1.8B-Chat-GGUF`
- HuggingFace: `BAAI/bge-m3`、`BAAI/bge-small-zh-v1.5`

下载哪一个量化版本由档位决定 (推荐 `Q4_K_M`)。

## 性能预期 (粗略基线)

用于在里程碑里判断"是不是哪里卡了"，不是承诺值。

| 档位 / 模型 | 输出 tok/s (粗) | 备注 |
|-------------|----------------|------|
| S / Qwen1.5-1.8B Q4 | 30–60 | M1 Metal |
| M / Qwen2.5-3B Q4 | 20–35 | M1 Metal |
| L / Qwen2.5-7B Q4 | 50–90 | 5070 CUDA, `-ngl 99` |
| XL / Qwen2.5-14B Q4 | 25–45 | 5070 CUDA, `-ngl 99` |

如果实测严重低于这个量级，先排查：是否启用了 GPU offload、`n_ctx` 是不是过大、是否在 streaming 之外做了同步阻塞。
