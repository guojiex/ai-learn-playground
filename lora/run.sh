#!/bin/bash

# 1. 确保 uv 已安装
if ! command -v uv &>/dev/null; then
    curl -LsSf https://astral.sh/uv/install.sh | sh
    source $HOME/.cargo/env
fi

VENV_DIR="venv"

# 2. 创建环境并安装依赖
if [ ! -d "$VENV_DIR" ]; then
    echo "🏗️ 正在创建 Python 3.11 环境..."
    uv venv $VENV_DIR --python 3.11
fi

source $VENV_DIR/bin/activate

echo "📦 正在安装依赖..."
uv pip install -r requirements.txt -i https://pypi.org/simple

echo "📦 正在精准同步 M1 适配环境..."
# 1. 先安装最新稳定版的 torch 及其配套组件
uv pip install "torch>=2.4" "torchvision" "torchaudio"

# 2. 安装其余依赖，并强制约束 numpy 版本
uv pip install -r requirements.txt "numpy<2" "huggingface_hub"

# 🔑 权限检查
if [ -z "$HF_TOKEN" ]; then
    echo "⚠️ 警告: 未检测到 HF_TOKEN 环境变量。"
    echo "如果稍后出现 401 错误，请先执行: export HF_TOKEN=你的转发Token"
fi

# 3. 运行程序
echo "🚀 启动演示..."
# 显式地将 Token 传递给 Python 环境
HF_TOKEN=$HF_TOKEN python main.py

deactivate