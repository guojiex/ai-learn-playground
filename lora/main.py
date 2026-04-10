"""LoRA \u5fae\u8c03\u5b9e\u6218 \u2014 \u4e3b\u5165\u53e3\u811a\u672c\u3002

\u4e32\u8054\u6240\u6709\u6f14\u793a\u6b65\u9aa4\uff0c\u4e5f\u53ef\u5355\u72ec\u8fd0\u884c\u6bcf\u4e2a step \u811a\u672c\u3002

Usage:
    python main.py           # \u8fd0\u884c\u5168\u90e8\u6b65\u9aa4
    python main.py 1         # \u53ea\u8fd0\u884c step1 (baseline)
    python main.py 2         # \u53ea\u8fd0\u884c step2 (LoRA inject)
    python main.py 3         # \u53ea\u8fd0\u884c step3 (training)
    python main.py 4         # \u53ea\u8fd0\u884c step4 (before/after)
    python main.py 5         # \u53ea\u8fd0\u884c step5 (evaluate)
    python main.py 6         # \u53ea\u8fd0\u884c step6 (3-model compare)
"""

import sys

from utils import EXPERIMENT_LABEL, DATA_PATH, ADAPTER_DIR

import step1_baseline
import step2_lora_inject
import step3_train
import step4_inference
import step5_evaluate
import step6_compare

STEPS = {
    "1": ("Baseline \u2014 \u901a\u7528\u6a21\u578b\u8868\u73b0", step1_baseline.main),
    "2": ("LoRA \u6ce8\u5165 \u2014 \u53c2\u6570\u6548\u7387\u5c55\u793a", step2_lora_inject.main),
    "3": ("\u5fae\u8c03\u8bad\u7ec3 \u2014 \u4e1a\u52a1\u6570\u636e\u6ce8\u5165", step3_train.main),
    "4": ("\u63a8\u7406\u5bf9\u6bd4 \u2014 Before vs After", step4_inference.main),
    "5": ("\u91cf\u5316\u8bc4\u6d4b \u2014 \u56f0\u60d1\u5ea6 + \u9ed1\u8bdd\u547d\u4e2d\u7387", step5_evaluate.main),
    "6": ("\u4e09\u6a21\u578b\u5bf9\u6bd4 \u2014 No/Small/Large LoRA", step6_compare.main),
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
        print('  \u4f4e\u6210\u672c\u6253\u9020\u201c\u61c2\u884c\u201d\u7684 AI\uff1a\u53f0\u7063\u96fb\u5546\u7db2\u7d05\u5834\u666f')
        print(f"  实验: {EXPERIMENT_LABEL} | 数据: {DATA_PATH.name} | 输出: {ADAPTER_DIR}")
        print("=" * 60)
        for key in sorted(STEPS.keys()):
            run_step(key)

    print("\n🎉 全部演示完成！")


if __name__ == "__main__":
    main()
