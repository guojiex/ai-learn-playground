#!/bin/bash
# LoRA 微调实战 — 大数据量实验 (200 条)
# 输出到 output/tw-affiliate-lora-large

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
uv pip install "torch>=2.4" "torchvision" "torchaudio"
uv pip install -r requirements.txt "numpy<2" "huggingface_hub"

# 🔑 权限检查
if [ -z "$HF_TOKEN" ]; then
    echo "⚠️ 警告: 未检测到 HF_TOKEN 环境变量。"
    echo "如果稍后出现 401 错误，请先执行: export HF_TOKEN=你的Token"
fi

# 3. 设置实验参数 — 大数据量 (200 条)
export LORA_EXPERIMENT="large-200"
export LORA_DATA_PATH="$(pwd)/data/tw_affiliate_large.jsonl"
export LORA_ADAPTER_DIR="$(pwd)/output/tw-affiliate-lora-large"
export LORA_EVAL_PATH="$(pwd)/data/eval_prompts_large.jsonl"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 实验: $LORA_EXPERIMENT"
echo "📂 训练数据: $LORA_DATA_PATH (200 条)"
echo "💾 适配器输出: $LORA_ADAPTER_DIR"
echo "📝 评测数据: $LORA_EVAL_PATH (10 prompts)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 4. 运行程序（支持指定步骤）
echo "🚀 启动大数据量实验..."
HF_TOKEN=$HF_TOKEN python main.py "$@"

deactivate
