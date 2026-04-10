#!/bin/bash
# LoRA 微调实战 — 三模型对比
# 对比: No LoRA / Small LoRA (30条) / Large LoRA (200条)
#
# 前置条件: 先分别运行两轮训练
#   bash run.sh 3        → 训练小数据适配器
#   bash run_large.sh 3  → 训练大数据适配器
# 然后运行本脚本进行对比

VENV_DIR="venv"

if [ ! -d "$VENV_DIR" ]; then
    echo "❌ 未找到 venv，请先运行 run.sh 或 run_large.sh 来安装依赖。"
    exit 1
fi

source $VENV_DIR/bin/activate

# 检查适配器是否存在
if [ ! -d "output/tw-affiliate-lora" ]; then
    echo "❌ 小数据适配器不存在，请先运行: bash run.sh 3"
    deactivate
    exit 1
fi

if [ ! -d "output/tw-affiliate-lora-large" ]; then
    echo "❌ 大数据适配器不存在，请先运行: bash run_large.sh 3"
    deactivate
    exit 1
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 三模型对比实验"
echo "   1. No LoRA (基座模型)"
echo "   2. Small LoRA (30 条数据)"
echo "   3. Large LoRA (200 条数据)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo "🚀 启动对比..."
HF_TOKEN=$HF_TOKEN python step6_compare.py

echo ""
echo "📋 对比报告已保存到: output/compare_report.json"

deactivate
