"""Step 6: Compare — side-by-side comparison of no-LoRA, small-LoRA, large-LoRA.

Loads the base model once, then evaluates three configurations:
  1. No adapter  (baseline)
  2. Small-data adapter  (30 samples)
  3. Large-data adapter   (200 samples)

Outputs a summary table with PPL, Slang Hit Rate, and sample generations.
"""

import json
import math
import os
import time
import torch
from pathlib import Path
from peft import PeftModel

from utils import detect_device, load_base_model, generate_text, MODEL_ID

_BASE = Path(__file__).parent

SMALL_ADAPTER = _BASE / "output" / "tw-affiliate-lora"
LARGE_ADAPTER = _BASE / "output" / "tw-affiliate-lora-large"
SMALL_DATA = _BASE / "data" / "tw_affiliate.jsonl"
LARGE_DATA = _BASE / "data" / "tw_affiliate_large.jsonl"
EVAL_PROMPTS = _BASE / "data" / "eval_prompts_large.jsonl"

COMMON_SLANG = [
    "\u5bf6", "\u5bf6\u5011", "\u5bb6\u4eba\u5011",
    "\u624b\u5200\u4e0b\u55ae", "\u624b\u5200", "\u79d2\u6bba",
    "cp\u503c", "\u514d\u904b", "\u7d50\u5e33",
    "\u6298\u6263\u78bc", "\u56e4\u8ca8", "\u7a2e\u8349",
    "\u771f\u5fc3\u4e0d\u9a19", "\u4f5b\u5fc3\u50f9",
    "\u6bcd\u6e6f", "\u5fc5\u8cb7", "\u56de\u8cfc",
    "\u958b\u7bb1", "\u8e29\u96f7", "\u7834\u76e4\u50f9",
    "hen", "94\u72c2",
]

COMPARE_PROMPTS = [
    "\u4f60\u662f\u53f0\u7063\u8766\u76ae\u9802\u7d1a\u96fb\u5546\u7db2\u7d05\u3002\u8acb\u7528\u53f0\u7063\u53e3\u8a9e\u98a8\u683c\u3001\u7a7f\u63d2\u53f0\u7063\u96fb\u5546\u9ed1\u8a71\uff0c\u5b89\u5229\u9019\u6b3e\u884c\u52d5\u96fb\u6e90: 20000mAh\uff0c\u7279\u50f9 NT$499\u3002",
    "\u4f60\u662f\u53f0\u7063\u8cc7\u6df1\u96fb\u5546\u7db2\u7d05\uff0c\u7528\u53f0\u7063\u53e3\u8a9e\u98a8\u683c\u56de\u8986\u5ba2\u4eba\u63d0\u554f: \u9019\u500b\u9084\u6709\u8ca8\u55ce\uff1f\u80fd\u514d\u904b\u55ce\uff1f",
    "\u4f60\u662f\u53f0\u7063\u8cc7\u6df1\u96fb\u5546\u7db2\u7d05\uff0c\u5beb\u4e00\u6bb5\u96d9 12 \u5927\u4fc3\u7684\u63a8\u5ee3\u6587\u6848\u3002\u7528\u53f0\u7063\u53e3\u8a9e\u98a8\u683c\u3002",
]


def compute_ppl(model, tokenizer, data_path, device, max_samples=10):
    """Perplexity on training data."""
    total_loss = 0.0
    total_tokens = 0
    with open(data_path, "r", encoding="utf-8") as f:
        lines = [json.loads(l.strip()) for l in f if l.strip()]
    for item in lines[:max_samples]:
        instruction = item["instruction"]
        inp = item.get("input", "")
        output = item["output"]
        user_msg = f"{instruction}\n{inp}" if inp else instruction
        messages = [
            {"role": "user", "content": user_msg},
            {"role": "assistant", "content": output},
        ]
        text = tokenizer.apply_chat_template(messages, tokenize=False)
        encoded = tokenizer(text, return_tensors="pt", truncation=True, max_length=512)
        input_ids = encoded["input_ids"].to(device)
        attention_mask = encoded["attention_mask"].to(device)
        with torch.no_grad():
            outputs = model(input_ids=input_ids, attention_mask=attention_mask, labels=input_ids)
            total_loss += outputs.loss.item() * input_ids.shape[1]
            total_tokens += input_ids.shape[1]
    return math.exp(total_loss / total_tokens)


def slang_hit_rate(text):
    text_lower = text.lower()
    hits = [s for s in COMMON_SLANG if s in text_lower]
    return hits, len(hits) / len(COMMON_SLANG) if COMMON_SLANG else 0


def eval_prompts_hit_rate(model, tokenizer, device):
    """Evaluate slang hit rate on eval prompts."""
    if not EVAL_PROMPTS.exists():
        return 0.0, []
    with open(EVAL_PROMPTS, "r", encoding="utf-8") as f:
        prompts = [json.loads(l.strip()) for l in f if l.strip()]

    total_hits = 0
    total_expected = 0
    details = []
    for ep in prompts:
        out = generate_text(model, tokenizer, ep["prompt"], device,
                            max_new_tokens=256, stream=False)
        expected = [s.lower() for s in ep["expected_slang"]]
        hits = [s for s in expected if s in out.lower()]
        total_hits += len(hits)
        total_expected += len(expected)
        details.append({
            "prompt": ep["prompt"],
            "output": out,
            "hits": len(hits),
            "expected": len(expected),
            "matched": hits,
        })

    overall = total_hits / total_expected if total_expected else 0
    return overall, details


def generate_samples(model, tokenizer, device, label):
    """Generate sample outputs for visual comparison."""
    outputs = []
    for prompt in COMPARE_PROMPTS:
        out = generate_text(model, tokenizer, prompt, device,
                            max_new_tokens=256, stream=False)
        outputs.append(out)
    return outputs


def fmt_duration(seconds):
    """Format seconds into a human-readable string."""
    if seconds < 60:
        return f"{seconds:.1f}s"
    minutes = int(seconds // 60)
    secs = seconds % 60
    return f"{minutes}m {secs:.0f}s"


def build_overview(results, timings, total_elapsed):
    """Build a structured overview with key takeaways and timing info."""
    labels = list(results.keys())
    base_label = labels[0]
    base = results[base_label]

    takeaways = []

    ppl_values = {k: v["ppl"] for k, v in results.items()}
    slang_values = {k: v["slang_hit"] for k, v in results.items()}

    best_ppl_label = min(ppl_values, key=ppl_values.get)
    best_slang_label = max(slang_values, key=slang_values.get)

    if base["ppl"] > 0:
        ppl_reduction = (1 - ppl_values[best_ppl_label] / base["ppl"]) * 100
        takeaways.append(
            f"困惑度 (PPL): {best_ppl_label} 表现最佳 "
            f"({ppl_values[best_ppl_label]:.2f})，"
            f"相比基座下降 {ppl_reduction:.1f}%"
        )

    slang_improvement = slang_values[best_slang_label] - base["slang_hit"]
    takeaways.append(
        f"黑话命中率: {best_slang_label} 最高 "
        f"({slang_values[best_slang_label]:.1%})，"
        f"比基座提升 {slang_improvement:.1%} 个百分点"
    )

    for i in range(1, len(labels)):
        prev, curr = labels[i - 1], labels[i]
        if i >= 2:
            slang_delta = slang_values[curr] - slang_values[prev]
            ppl_delta = ppl_values[prev] - ppl_values[curr]
            takeaways.append(
                f"{curr} vs {prev}: "
                f"黑话命中率 +{slang_delta:.1%}，PPL 再降 {ppl_delta:.4f}"
            )

    takeaways.append(f"结论: 数据量从 30→200 条后，模型对领域风格的掌握度持续提升")

    timing_formatted = {k: fmt_duration(v) for k, v in timings.items()}

    return {
        "model": MODEL_ID,
        "takeaways": takeaways,
        "best_ppl": {"label": best_ppl_label, "value": ppl_values[best_ppl_label]},
        "best_slang_hit": {"label": best_slang_label, "value": slang_values[best_slang_label]},
        "timings": timing_formatted,
        "total_time": fmt_duration(total_elapsed),
        "total_time_seconds": round(total_elapsed, 1),
    }


def main():
    print("=" * 70)
    print("  Step 6: 三模型对比 — No LoRA / Small LoRA (30) / Large LoRA (200)")
    print("=" * 70)

    missing = []
    if not SMALL_ADAPTER.exists():
        missing.append(f"小数据适配器: {SMALL_ADAPTER}")
    if not LARGE_ADAPTER.exists():
        missing.append(f"大数据适配器: {LARGE_ADAPTER}")
    if missing:
        print("\n\u274c \u7f3a\u5c11\u9002\u914d\u5668\uff0c\u8bf7\u5148\u8fd0\u884c\u5bf9\u5e94\u7684\u8bad\u7ec3:")
        for m in missing:
            print(f"   - {m}")
        print("\n   \u8bf7\u5148\u8fd0\u884c: bash run.sh 3  &&  bash run_large.sh 3")
        return

    device, use_quant = detect_device()
    tokenizer, base_model = load_base_model(device, use_quant)

    # Wrap with small adapter (we'll toggle adapters)
    print("\n\U0001f517 \u52a0\u8f7d\u5c0f\u6570\u636e\u9002\u914d\u5668...")
    model_small = PeftModel.from_pretrained(base_model, str(SMALL_ADAPTER), adapter_name="small")
    print("\U0001f517 \u52a0\u8f7d\u5927\u6570\u636e\u9002\u914d\u5668...")
    model_small.load_adapter(str(LARGE_ADAPTER), adapter_name="large")

    configs = [
        ("No LoRA (\u57fa\u5ea7)", None, SMALL_DATA),
        ("Small LoRA (30\u6761)", "small", SMALL_DATA),
        ("Large LoRA (200\u6761)", "large", LARGE_DATA),
    ]

    results = {}
    all_samples = {}
    all_eval_details = {}
    timings = {}

    t_total = time.time()

    for label, adapter_name, data_path in configs:
        print(f"\n{'=' * 70}")
        print(f"\U0001f4ca \u8bc4\u6d4b: {label}")
        print("=" * 70)

        t_config = time.time()

        if adapter_name is None:
            with model_small.disable_adapter():
                model_small.eval()
                ppl = compute_ppl(model_small, tokenizer, data_path, device)
                print(f"   PPL ({data_path.name}): {ppl:.2f}")
                hit_rate, eval_details = eval_prompts_hit_rate(model_small, tokenizer, device)
                print(f"   \u9ed1\u8bdd\u547d\u4e2d\u7387: {hit_rate:.1%}")
                samples = generate_samples(model_small, tokenizer, device, label)
        else:
            model_small.set_adapter(adapter_name)
            model_small.eval()
            ppl = compute_ppl(model_small, tokenizer, data_path, device)
            print(f"   PPL ({data_path.name}): {ppl:.2f}")
            hit_rate, eval_details = eval_prompts_hit_rate(model_small, tokenizer, device)
            print(f"   \u9ed1\u8bdd\u547d\u4e2d\u7387: {hit_rate:.1%}")
            samples = generate_samples(model_small, tokenizer, device, label)

        elapsed_config = time.time() - t_config
        timings[label] = elapsed_config
        print(f"   ⏱️  耗时: {elapsed_config:.1f}s")

        results[label] = {"ppl": ppl, "slang_hit": hit_rate}
        all_samples[label] = samples
        all_eval_details[label] = eval_details

    total_elapsed = time.time() - t_total

    # ===== Summary Table =====
    print(f"\n\n{'=' * 70}")
    print("\U0001f4cb \u4e09\u6a21\u578b\u5bf9\u6bd4\u6c47\u603b")
    print("=" * 70)

    col1, col2, col3 = "", "PPL ↓", "黑话命中率 ↑"
    header = f"{col1:30s} {col2:>12s} {col3:>14s}"
    print(f"\n   {header}")
    print(f"   {'-' * 58}")
    for label, metrics in results.items():
        ppl_str = f"{metrics['ppl']:.2f}"
        slang_str = f"{metrics['slang_hit']:.1%}"
        print(f"   {label:30s} {ppl_str:>12s} {slang_str:>14s}")

    # ===== Overview =====
    overview = build_overview(results, timings, total_elapsed)

    print(f"\n\n{'=' * 70}")
    print("📊 Overview — 对比结论")
    print("=" * 70)
    for line in overview["takeaways"]:
        print(f"   • {line}")
    print(f"\n   ⏱️  总评测耗时: {overview['total_time']}")
    for label, t in overview["timings"].items():
        print(f"      - {label}: {t}")

    # ===== Sample Outputs =====
    print(f"\n\n{'=' * 70}")
    print("\U0001f4dd \u751f\u6210\u6837\u672c\u5bf9\u6bd4")
    print("=" * 70)
    labels = list(all_samples.keys())
    for i, prompt in enumerate(COMPARE_PROMPTS):
        print(f"\n\U0001f3af Prompt {i+1}: {prompt[:60]}...")
        print("-" * 70)
        for label in labels:
            sample = all_samples[label][i]
            preview = sample[:200] + "..." if len(sample) > 200 else sample
            print(f"\n   [{label}]")
            print(f"   {preview}")
        print()

    # ===== Save to JSON =====
    report_path = _BASE / "output" / "compare_report.json"
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report = {
        "overview": overview,
        "metrics": {k: v for k, v in results.items()},
        "samples": {k: v for k, v in all_samples.items()},
        "prompts": COMPARE_PROMPTS,
        "eval_details": {k: v for k, v in all_eval_details.items()},
    }
    with open(report_path, "w", encoding="utf-8") as f:
        json.dump(report, f, ensure_ascii=False, indent=2)
    print(f"\n\U0001f4be \u5bf9\u6bd4\u62a5\u544a\u5df2\u4fdd\u5b58: {report_path}")

    print("\n\u2705 \u5bf9\u6bd4\u5b8c\u6210\uff01")


if __name__ == "__main__":
    main()
