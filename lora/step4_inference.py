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
            generate_text(tuned_model, tokenizer, prompt, device)

        print("\n🟢 After（微调后 — 挂载 LoRA 适配器）:")
        print("-" * 50)
        generate_text(tuned_model, tokenizer, prompt, device)

    print(f"\n{'=' * 60}")
    print("\U0001f4a1 \u5173\u952e\u89c2\u5bdf:")
    print("   - Before: \u56de\u590d\u504f\u6b63\u5f0f\u3001\u50cf\u5ba2\u670d\u8bdd\u672f\uff0c\u7f3a\u5c11\u53f0\u6e7e\u7535\u5546\u6c1b\u56f4")
    print("   - After:  \u81ea\u7136\u7a7f\u63d2 \u624b\u5200\u4e0b\u55ae/CP\u503c/\u514d\u904b/\u79d2\u6bba/\u6bcd\u6e6f/\u771f\u5fc3\u4e0d\u9a19 \u7b49\u53f0\u6e7e\u9ed1\u8bdd")
    print("   - \u4ec5\u7528 30 \u6761\u6837\u672c\u6570\u636e + \u51e0\u5206\u949f\u8bad\u7ec3\u5373\u53ef\u770b\u5230\u98ce\u683c\u8f6c\u53d8\uff01")
    print("=" * 60)
    print("\n✅ 演示完成！")


if __name__ == "__main__":
    main()
