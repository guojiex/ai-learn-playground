"""LoRA 微调实战 — 主入口脚本。

串联所有演示步骤，也可单独运行每个 step 脚本。

Usage:
    python main.py           # 运行全部步骤
    python main.py 1         # 只运行 step1 (baseline)
    python main.py 2         # 只运行 step2 (LoRA inject)
    python main.py 3         # 只运行 step3 (training)
    python main.py 4         # 只运行 step4 (before/after)
"""

import sys

import step1_baseline
import step2_lora_inject
import step3_train
import step4_inference

STEPS = {
    "1": ("Baseline — 通用模型表现", step1_baseline.main),
    "2": ("LoRA 注入 — 参数效率展示", step2_lora_inject.main),
    "3": ("微调训练 — 业务数据注入", step3_train.main),
    "4": ("推理对比 — Before vs After", step4_inference.main),
}


def run_step(key):
    name, func = STEPS[key]
    print(f"\n{'#' * 60}")
    print(f"# 🚀 正在运行 Step {key}: {name}")
    print(f"{'#' * 60}\n")
    func()


def main():
    if len(sys.argv) > 1:
        key = sys.argv[1]
        if key in STEPS:
            run_step(key)
        else:
            print(f"❌ 未知步骤: {key}")
            print(f"   可用步骤: {', '.join(STEPS.keys())}")
            sys.exit(1)
    else:
        print("=" * 60)
        print("  LoRA 微调实战 — 全流程演示")
        print('  \u4f4e\u6210\u672c\u6253\u9020\u201c\u61c2\u884c\u201d\u7684 AI\uff1a\u4e1c\u5357\u4e9a\u7535\u5546\u63a8\u5ba2\u573a\u666f')
        print("=" * 60)
        for key in sorted(STEPS.keys()):
            run_step(key)

    print("\n🎉 全部演示完成！")


if __name__ == "__main__":
    main()
