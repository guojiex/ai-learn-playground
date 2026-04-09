"""Step 1: Baseline -- show how a generic model struggles with local context.

After running, you'll see that generic models produce overly formal responses,
lacking local slang and the casual tone expected in SEA affiliate marketing.
"""

from utils import detect_device, load_base_model, generate_text, DEMO_PROMPTS


def main():
    print("=" * 60)
    print("📌 Step 1: Baseline — 微调前的通用模型表现")
    print("=" * 60)

    device, use_quant = detect_device()
    tokenizer, model = load_base_model(device, use_quant)

    param_count = sum(p.numel() for p in model.parameters())
    print(f"📊 模型总参数量: {param_count:,}")

    print("\n" + "=" * 60)
    print("🔍 通用模型推理结果（微调前）")
    print("=" * 60)

    for i, prompt in enumerate(DEMO_PROMPTS, 1):
        print(f"\n--- Prompt {i} ---")
        print(f"📝 {prompt}\n")
        result = generate_text(model, tokenizer, prompt, device)
        print(f"🤖 模型输出:\n{result}")
        print("-" * 50)

    print('\n\u26a0\ufe0f  \u89c2\u5bdf\uff1a\u901a\u7528\u6a21\u578b\u7684\u56de\u590d\u662f\u5426\u6709\u201c\u7ffb\u8bd1\u8154\u201d\uff1f\u662f\u5426\u7f3a\u5c11\u672c\u5730\u7535\u5546\u9ed1\u8bdd\uff1f')


if __name__ == "__main__":
    main()
