# ai-learn-playground

本仓库用于沉淀本地大模型、Agent、LoRA 微调等学习实验。

## 一条命令启动本地大模型 + 网页

在仓库根目录执行：

```powershell
.\agent-lab\tools\run.ps1 local-web
```

启动成功后打开：

```text
http://127.0.0.1:8090/
```

这条命令会自动：

- 创建并复用 `agent-lab/.venv` 虚拟环境
- 在虚拟环境里安装 Python 推理依赖
- 启动 Python 本地大模型 OpenAI 兼容服务
- 等待模型服务可用
- 启动 `agent-lab` Web UI
- Web 退出时自动清理后台进程

默认模型：

```text
Qwen/Qwen2.5-3B-Instruct
```

依赖文件位于：

```text
agent-lab/scripts/python-openai-server/requirements.txt
```

如果只是想先验证网页和服务能不能启动，但不想立即加载模型：

```powershell
.\agent-lab\tools\run.ps1 local-web -Lazy
```

## 项目目录

- `agent-lab/`：从零手写 Agent 学习实验室。主程序使用 Go，LLM 通过本地 OpenAI 兼容服务接入。
- `lora/`：Python LoRA / QLoRA 微调实战，使用 `transformers` 本地加载 Qwen 模型并进行训练、推理和评测。
