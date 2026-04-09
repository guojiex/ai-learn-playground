"""Step 4: Inference -- load the fine-tuned adapter and compare Before/After.

The highlight of the demo: same prompt, drastically different outputs,
showing how LoRA transforms a generic model into a local e-commerce expert.
"""

from peft import PeftModel

from utils import (
    detect_device, load_base_model, generate_text,
    ADAPTER_DIR, DEMO_PROMPTS,
)


def main():
    print("=" * 60)
    print("📌 Step 4: Before vs After — 微调效果对比")
    print("=" * 60)

    if not ADAPTER_DIR.exists():
        print(f"❌ 未找到训练好的适配器: {ADAPTER_DIR}")
        print("   请先运行 step3_train.py 进行训练。")
        return

    device, use_quant = detect_device()
    tokenizer, base_model = load_base_model(device, use_quant)

    print("🔗 加载 LoRA 适配器...")
    tuned_model = PeftModel.from_pretrained(base_model, str(ADAPTER_DIR))
    tuned_model.eval()
    print(f"   已加载: {ADAPTER_DIR}")

    for i, prompt in enumerate(DEMO_PROMPTS, 1):
        print(f"\n{'=' * 60}")
        print(f"🎯 测试 Prompt {i}:")
        print(f"   {prompt}")
        print("=" * 60)

        print("\n🔴 Before（通用模型 — 无 LoRA）:")
        print("-" * 50)
        with tuned_model.disable_adapter():
            before = generate_text(tuned_model, tokenizer, prompt, device)
        print(before)

        print("\n🟢 After（微调后 — 挂载 LoRA 适配器）:")
        print("-" * 50)
        after = generate_text(tuned_model, tokenizer, prompt, device)
        print(after)

    print(f"\n{'=' * 60}")
    print("💡 关键观察:")
    print("   - Before: 回复偏正式、像客服话术，缺少本地电商氛围")
    print("   - After:  中文为主体，自然穿插 Bestie/Spill/Checkout/Gratis Ongkir 等印尼黑话")
    print("   - 仅用 10 条样本数据 + 几分钟训练即可看到风格转变！")
    print("=" * 60)
    print("\n✅ 演示完成！")


if __name__ == "__main__":
    main()
