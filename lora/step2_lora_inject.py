"""Step 2: LoRA Injection — 注入 LoRA 适配器，展示参数效率。

演示核心卖点：只需训练不到 1% 的参数，即可对大模型进行微调。
"""

import os
from peft import LoraConfig, get_peft_model, TaskType
from utils import detect_device, load_base_model


LORA_PROFILES = {
    "default": dict(
        r=16,
        lora_alpha=32,
        lora_dropout=0.1,
        target_modules=["q_proj", "v_proj", "k_proj", "o_proj"],
    ),
    "large": dict(
        r=16,
        lora_alpha=32,
        lora_dropout=0.05,
        target_modules=["q_proj", "v_proj", "k_proj", "o_proj"],
    ),
}


def get_lora_config():
    """Return the LoRA config, auto-selecting profile based on LORA_EXPERIMENT."""
    experiment = os.environ.get("LORA_EXPERIMENT", "default")
    profile_key = "large" if "large" in experiment else "default"
    profile = LORA_PROFILES[profile_key]
    return LoraConfig(
        task_type=TaskType.CAUSAL_LM,
        inference_mode=False,
        **profile,
    )


def main():
    print("=" * 60)
    print('\U0001f4cc Step 2: LoRA \u6ce8\u5165 \u2014 \u7ed9\u6a21\u578b\u8d34\u4e0a\u201c\u900f\u660e\u4fbf\u5229\u8d34\u201d')
    print("=" * 60)

    device, use_quant = detect_device()
    tokenizer, model = load_base_model(device, use_quant)

    if use_quant:
        from peft import prepare_model_for_kbit_training
        model = prepare_model_for_kbit_training(model)

    total_before = sum(p.numel() for p in model.parameters())
    print(f"\n📊 注入前 — 模型总参数量: {total_before:,}")

    peft_config = get_lora_config()
    model = get_peft_model(model, peft_config)

    print("\n🛠️  LoRA 配置:")
    print(f"   Rank (r)     = {peft_config.r}")
    print(f"   Alpha        = {peft_config.lora_alpha}")
    print(f"   Alpha/r      = {peft_config.lora_alpha / peft_config.r}")
    print(f"   Dropout      = {peft_config.lora_dropout}")
    print(f"   Target       = {peft_config.target_modules}")

    print("\n📊 参数效率对比:")
    model.print_trainable_parameters()

    trainable = sum(p.numel() for p in model.parameters() if p.requires_grad)
    total = sum(p.numel() for p in model.parameters())
    print(f"\n   全量微调需更新: {total:>12,} 参数")
    print(f"   LoRA 只需更新: {trainable:>12,} 参数")
    print(f"   压缩比:         {trainable / total * 100:.2f}%")

    print("\n✅ LoRA 适配器注入成功！下一步: 使用业务数据进行微调训练。")


if __name__ == "__main__":
    main()
