# ai-learn-playground

本仓库用于沉淀本地大模型、Agent、LoRA 微调等学习实验。

## 项目目录

- `agent-lab/`：从零手写 Agent 学习实验室。主程序使用 Go，LLM 通过本地 OpenAI 兼容服务接入。
- `lora/`：Python LoRA / QLoRA 微调实战，使用 `transformers` 本地加载 Qwen 模型并进行训练、推理和评测。

## agent-lab 快速启动

`agent-lab` 不直接把模型写死在 Go 进程里，而是统一连接本地 OpenAI 兼容服务。可选后端包括：

- fake-openai：仓库内置模拟服务，不需要真实模型，适合快速跑通 UI 和流程。
- Python 本地模型服务：仓库内置 `transformers` 服务，像 `lora/` 一样在 Python 进程内加载本地模型。
- llama.cpp / Ollama：更成熟的 OpenAI 兼容本地模型后端。

### 方式一：fake-openai 快速跑通

```powershell
.\agent-lab\tools\run.ps1 demo-web
```

然后打开：

```text
http://127.0.0.1:8090/
```

### 方式二：Python 本地模型服务

先安装 Python 推理依赖：

```powershell
python -m pip install torch transformers accelerate
```

启动 Python OpenAI 兼容服务：

```powershell
.\agent-lab\tools\run.ps1 py-openai
```

默认加载 `Qwen/Qwen1.5-1.8B-Chat`，监听：

```text
http://127.0.0.1:18080/v1
```

如果只想先验证服务启动、不立即加载模型：

```powershell
.\agent-lab\tools\run.ps1 py-openai -Lazy
```

指定模型或设备：

```powershell
.\agent-lab\tools\run.ps1 py-openai -PyModel "Qwen/Qwen1.5-1.8B-Chat" -PyDevice cuda
```

另开一个终端，让 `agent-lab` 连接这个 Python 服务：

```powershell
$env:OPENAI_BASE_URL="http://127.0.0.1:18080/v1"
$env:OPENAI_API_KEY="sk-local"
$env:AGENTLAB_PROFILE="S"
$env:AGENTLAB_MODEL_CHAT="qwen1.5-1.8b-chat"
go run .\agent-lab\cmd\chat -m "你好"
```

Python 服务代码位于：

```text
agent-lab/scripts/python-openai-server/main.py
```

支持接口：

- `GET /healthz`
- `GET /v1/models`
- `POST /v1/chat/completions`

### 方式三：Web UI 连接 Python 本地模型

终端 A：

```powershell
.\agent-lab\tools\run.ps1 py-openai
```

终端 B：

```powershell
$env:OPENAI_BASE_URL="http://127.0.0.1:18080/v1"
$env:OPENAI_API_KEY="sk-local"
$env:AGENTLAB_PROFILE="S"
$env:AGENTLAB_MODEL_CHAT="qwen1.5-1.8b-chat"
go run .\agent-lab\cmd\web
```

浏览器打开：

```text
http://127.0.0.1:8090/
```

## 说明

当前 Python 本地模型服务主要用于本地对话体验和 M0/M1/M3 阶段验证。后续如果要稳定跑 tool calling、RAG、多 Agent 和更复杂的 OpenAI 协议兼容能力，建议优先使用 llama.cpp 或 Ollama。
