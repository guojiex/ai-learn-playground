"""Shared utilities for LoRA fine-tuning demo."""

import torch
from pathlib import Path

MODEL_ID = "Qwen/Qwen1.5-1.8B-Chat"
ADAPTER_DIR = Path(__file__).parent / "output" / "sea-affiliate-lora"
DATA_PATH = Path(__file__).parent / "data" / "sea_affiliate.jsonl"

DEMO_PROMPTS = [
    "你是印尼 TikTok 顶级推客。请用中文为主、穿插印尼本地黑话的风格，安利这款充电宝: Powerbank 20000mAh, 仅售 100k IDR。",
    "作为印尼资深电商推客，用中文为主、融入印尼电商表达的风格，回复客户提问: 这个还有货吗？能包邮吗？",
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


def generate_text(model, tokenizer, prompt, device, max_new_tokens=150):
    """Generate text from a prompt using the Qwen chat template."""
    messages = [{"role": "user", "content": prompt}]
    text = tokenizer.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
    inputs = tokenizer(text, return_tensors="pt").to(device)

    with torch.no_grad():
        output_tokens = model.generate(
            **inputs,
            max_new_tokens=max_new_tokens,
            min_new_tokens=30,
            temperature=0.7,
            top_p=0.9,
            do_sample=True,
        )

    new_tokens = output_tokens[0][inputs["input_ids"].shape[1]:]
    return tokenizer.decode(new_tokens, skip_special_tokens=True).strip()
