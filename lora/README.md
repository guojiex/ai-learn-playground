# LoRA 微调实战 — 台灣電商網紅場景

用 LoRA / QLoRA 低成本微调大语言模型，让通用模型学会台湾电商网红的说话风格。

> 配套 PPT：`docs/LoRA微调实战技术分享_improved.pptx`

## 项目结构

```
lora/
├── run.sh                  # 小数据实验入口 (30 条)
├── run_large.sh            # 大数据实验入口 (200 条)
├── run_compare.sh          # 三模型对比入口
├── main.py                 # 主调度器 (step 1-6)
├── utils.py                # 公共工具 (模型加载、生成、环境变量配置)
├── step1_baseline.py       # 基座模型表现 (微调前)
├── step2_lora_inject.py    # LoRA 注入 & 参数效率展示
├── step3_train.py          # 微调训练
├── step4_inference.py      # Before vs After 推理对比
├── step5_evaluate.py       # 量化评测 (PPL + 黑话命中率)
├── step6_compare.py        # 三模型对比 (No LoRA / Small / Large)
├── requirements.txt        # Python 依赖
├── data/
│   ├── tw_affiliate.jsonl          # 小数据集 (30 条)
│   ├── tw_affiliate_large.jsonl    # 大数据集 (200 条)
│   ├── eval_prompts.jsonl          # 评测 prompts (5 条)
│   └── eval_prompts_large.jsonl    # 评测 prompts (10 条)
├── output/                         # 训练产出 (自动生成)
│   ├── tw-affiliate-lora/          # 小数据适配器
│   ├── tw-affiliate-lora-large/    # 大数据适配器
│   └── compare_report.json         # 对比报告
├── docs/                           # PPT & PDF
└── scripts/pptx_builders/          # PPT 构建脚本 (可选)
```

## 快速开始

### 环境要求

- Python 3.11+
- macOS (Apple Silicon MPS) 或 Linux (NVIDIA GPU + CUDA)
- 约 8GB 内存 / 显存

### 一键运行 — 小数据实验 (30 条)

```bash
cd lora
bash run.sh          # 运行全部 step 1-5
bash run.sh 3        # 只运行训练
bash run.sh 4        # 只运行推理对比
```

### 一键运行 — 大数据实验 (200 条)

```bash
bash run_large.sh          # 运行全部 step 1-5
bash run_large.sh 3        # 只运行训练 (~2000 steps, 需要更长时间)
```

### 三模型对比

先分别完成两轮训练，再运行对比：

```bash
# 1. 训练小数据 LoRA
bash run.sh 3

# 2. 训练大数据 LoRA
bash run_large.sh 3

# 3. 对比三种模型
bash run_compare.sh
```

对比输出一张汇总表：

```
                               PPL ↓    黑话命中率 ↑
--------------------------------------------------------------
No LoRA (基座)                 ~720          ~10%
Small LoRA (30条)              ~1.08         ~10%
Large LoRA (200条)              ???           ???
```

以及 3 个 prompt 的生成样本对比，完整报告保存到 `output/compare_report.json`。

## 单步运行

每个 step 也可以独立运行：

```bash
source venv/bin/activate
python step1_baseline.py       # 看基座模型表现
python step2_lora_inject.py    # 看 LoRA 参数效率
python step3_train.py          # 训练
python step4_inference.py      # Before vs After
python step5_evaluate.py       # PPL + 黑话命中率
python step6_compare.py        # 三模型对比
```

## 环境变量配置

所有路径通过环境变量控制，方便自定义实验：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LORA_DATA_PATH` | `data/tw_affiliate.jsonl` | 训练数据路径 |
| `LORA_ADAPTER_DIR` | `output/tw-affiliate-lora` | 适配器保存/加载路径 |
| `LORA_EVAL_PATH` | `data/eval_prompts.jsonl` | 评测 prompts 路径 |
| `LORA_EXPERIMENT` | `default` | 实验标签 (显示在输出中) |
| `HF_TOKEN` | (无) | HuggingFace Token (可选) |

示例 — 用自定义数据训练：

```bash
export LORA_DATA_PATH="$(pwd)/data/my_custom_data.jsonl"
export LORA_ADAPTER_DIR="$(pwd)/output/my-experiment"
export LORA_EXPERIMENT="custom-v1"
python main.py 3   # 训练
python main.py 4   # 推理对比
```

## 技术栈

| 组件 | 版本 |
|------|------|
| 底座模型 | Qwen/Qwen1.5-1.8B-Chat |
| 微调方法 | LoRA (r=16, alpha=32) |
| 量化 | QLoRA 4-bit NF4 (CUDA only) |
| 框架 | transformers + peft + bitsandbytes |

## 评测指标

- **困惑度 (Perplexity, PPL)** — 模型对领域数据的"理解度"，越低越好
- **黑话命中率 (Slang Hit Rate)** — 生成内容中台湾电商术语的覆盖率，越高越好
- 生产环境建议加入：LLM-as-Judge + 人工评测 + A/B 测试
