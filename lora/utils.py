"""Shared utilities for LoRA fine-tuning demo."""

import torch
from pathlib import Path

MODEL_ID = "Qwen/Qwen1.5-1.8B-Chat"
ADAPTER_DIR = Path(__file__).parent / "output" / "tw-affiliate-lora"
DATA_PATH = Path(__file__).parent / "data" / "tw_affiliate.jsonl"

DEMO_PROMPTS = [
    "\u4f60\u662f\u53f0\u7063\u8766\u76ae\u9802\u7d1a\u96fb\u5546\u7db2\u7d05\u3002\u8acb\u7528\u53f0\u7063\u53e3\u8a9e\u98a8\u683c\u3001\u7a7f\u63d2\u53f0\u7063\u96fb\u5546\u9ed1\u8a71\uff0c\u5b89\u5229\u9019\u6b3e\u884c\u52d5\u96fb\u6e90: 20000mAh\uff0c\u7279\u50f9 NT$499\u3002",
    "\u4f60\u662f\u53f0\u7063\u8cc7\u6df1\u96fb\u5546\u7db2\u7d05\uff0c\u7528\u53f0\u7063\u53e3\u8a9e\u98a8\u683c\u56de\u8986\u5ba2\u4eba\u63d0\u554f: \u9019\u500b\u9084\u6709\u8ca8\u55ce\uff1f\u80fd\u514d\u904b\u55ce\uff1f",
]


def detect_device():
    """Detect best available device and return (device, use_quantization) tuple."""
    if torch.cuda.is_available():
        device = torch.device("cuda")
        print("🖥️  检测到 NVIDIA GPU，使用 CUDA 加速 + 4-bit 量化")
        return device, True
    if torch.backends.mps.is_available():
        device = torch.device("mps")
        print("🍏 检测到 Apple Silicon，使用 MPS 加速（跳过量化）")
        return device, False
    print("💻 未检测到 GPU，使用 CPU（跳过量化）")
    return torch.device("cpu"), False


def load_base_model(device, use_quantization=False):
    """Load tokenizer and base model with optional 4-bit quantization (QLoRA)."""
    from transformers import AutoModelForCausalLM, AutoTokenizer

    print(f"📦 正在加载模型: {MODEL_ID}...")
    tokenizer = AutoTokenizer.from_pretrained(MODEL_ID, trust_remote_code=True)

    load_kwargs = dict(trust_remote_code=True, low_cpu_mem_usage=True)

    if use_quantization:
        from transformers import BitsAndBytesConfig
        bnb_config = BitsAndBytesConfig(
            load_in_4bit=True,
            bnb_4bit_quant_type="nf4",
            bnb_4bit_compute_dtype=torch.bfloat16,
        )
        load_kwargs["quantization_config"] = bnb_config
        load_kwargs["device_map"] = "auto"
        print("⚡ 已启用 QLoRA 4-bit NF4 量化")
    else:
        load_kwargs["dtype"] = torch.bfloat16

    model = AutoModelForCausalLM.from_pretrained(MODEL_ID, **load_kwargs)

    if not use_quantization:
        model = model.to(device)

    return tokenizer, model


def generate_text(model, tokenizer, prompt, device, max_new_tokens=512, stream=True):
    """Generate text from a prompt using the Qwen chat template."""
    from transformers import TextStreamer
    import sys

    messages = [{"role": "user", "content": prompt}]
    text = tokenizer.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
    inputs = tokenizer(text, return_tensors="pt").to(device)

    streamer = TextStreamer(tokenizer, skip_prompt=True, skip_special_tokens=True) if stream else None

    with torch.no_grad():
        output_tokens = model.generate(
            **inputs,
            max_new_tokens=max_new_tokens,
            min_new_tokens=30,
            temperature=0.7,
            top_p=0.9,
            do_sample=True,
            streamer=streamer,
        )

    new_tokens = output_tokens[0][inputs["input_ids"].shape[1]:]
    result = tokenizer.decode(new_tokens, skip_special_tokens=True).strip()

    if len(new_tokens) >= max_new_tokens:
        print("\n... (reached token limit, truncated)")

    if not stream:
        print(result)

    return result
