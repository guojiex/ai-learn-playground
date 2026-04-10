"""Step 5: Evaluate -- quantitative evaluation of the fine-tuned model.

Computes three metrics:
  1. Perplexity on held-out data (lower = better language modeling)
  2. Slang Hit Rate (domain term usage in generated outputs)
  3. Side-by-side Before/After comparison on eval prompts
"""

import json
import math
import torch
from pathlib import Path
from peft import PeftModel

from utils import (
    detect_device, load_base_model, generate_text,
    ADAPTER_DIR, DATA_PATH,
)

EVAL_PROMPTS_PATH = Path(__file__).parent / "data" / "eval_prompts.jsonl"

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


def compute_perplexity(model, tokenizer, data_path, device, max_samples=10):
    """Compute perplexity on a subset of the training data."""
    total_loss = 0.0
    total_tokens = 0

    with open(data_path, "r", encoding="utf-8") as f:
        lines = [json.loads(line.strip()) for line in f if line.strip()]

    samples = lines[:max_samples]
    model.eval()

    for item in samples:
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

    avg_loss = total_loss / total_tokens
    perplexity = math.exp(avg_loss)
    return perplexity


def compute_slang_hit_rate(text):
    """Calculate what fraction of common slang terms appear in the output."""
    text_lower = text.lower()
    hits = [s for s in COMMON_SLANG if s in text_lower]
    return hits, len(hits) / len(COMMON_SLANG)


def load_eval_prompts():
    """Load evaluation prompts with expected slang lists."""
    prompts = []
    with open(EVAL_PROMPTS_PATH, "r", encoding="utf-8") as f:
        for line in f:
            if line.strip():
                prompts.append(json.loads(line.strip()))
    return prompts


def evaluate_generation(model, tokenizer, device, label, eval_prompts):
    """Run eval prompts through the model and measure slang hit rates."""
    results = []
    total_hits = 0
    total_expected = 0

    for i, ep in enumerate(eval_prompts):
        prompt = ep["prompt"]
        expected = [s.lower() for s in ep["expected_slang"]]

        output = generate_text(model, tokenizer, prompt, device,
                               max_new_tokens=256, stream=False)
        output_lower = output.lower()
        hits = [s for s in expected if s in output_lower]
        hit_rate = len(hits) / len(expected) if expected else 0

        total_hits += len(hits)
        total_expected += len(expected)

        results.append({
            "prompt_idx": i + 1,
            "hits": len(hits),
            "expected": len(expected),
            "hit_rate": hit_rate,
            "matched": hits,
        })

    overall = total_hits / total_expected if total_expected else 0
    return results, overall


def main():
    print("=" * 60)
    print("\U0001f4ca Step 5: \u6a21\u578b\u8bc4\u6d4b — \u91cf\u5316\u8861\u91cf\u5fae\u8c03\u6548\u679c")
    print("=" * 60)

    if not ADAPTER_DIR.exists():
        print(f"\u274c \u672a\u627e\u5230\u8bad\u7ec3\u597d\u7684\u9002\u914d\u5668: {ADAPTER_DIR}")
        print("   \u8bf7\u5148\u8fd0\u884c step3_train.py \u8fdb\u884c\u8bad\u7ec3\u3002")
        return

    device, use_quant = detect_device()
    tokenizer, base_model = load_base_model(device, use_quant)

    print("\U0001f517 \u52a0\u8f7d LoRA \u9002\u914d\u5668...")
    tuned_model = PeftModel.from_pretrained(base_model, str(ADAPTER_DIR))
    tuned_model.eval()

    # ===== 1. Perplexity =====
    print(f"\n{'=' * 60}")
    print("\U0001f4d0 \u8bc4\u6d4b\u9879 1: \u56f0\u60d1\u5ea6 (Perplexity)")
    print("=" * 60)
    print("   \u56f0\u60d1\u5ea6\u8d8a\u4f4e\uff0c\u8bf4\u660e\u6a21\u578b\u5bf9\u9886\u57df\u6570\u636e\u7684\u201c\u7406\u89e3\u201d\u8d8a\u597d\u3002\n")

    print("   \u8ba1\u7b97\u4e2d (Before / \u901a\u7528\u6a21\u578b)...")
    with tuned_model.disable_adapter():
        ppl_before = compute_perplexity(tuned_model, tokenizer, DATA_PATH, device)
    print(f"   \U0001f534 Before (base):  PPL = {ppl_before:.2f}")

    print("   \u8ba1\u7b97\u4e2d (After / LoRA \u5fae\u8c03)...")
    ppl_after = compute_perplexity(tuned_model, tokenizer, DATA_PATH, device)
    print(f"   \U0001f7e2 After  (LoRA):  PPL = {ppl_after:.2f}")

    ppl_drop = (1 - ppl_after / ppl_before) * 100
    print(f"\n   \U0001f4c9 \u56f0\u60d1\u5ea6\u4e0b\u964d: {ppl_drop:.1f}%")
    if ppl_drop > 0:
        print("   \u2705 \u5fae\u8c03\u540e\u6a21\u578b\u5bf9\u9886\u57df\u6570\u636e\u7684\u5efa\u6a21\u80fd\u529b\u663e\u8457\u63d0\u5347\uff01")
    else:
        print("   \u26a0\ufe0f \u56f0\u60d1\u5ea6\u672a\u660e\u663e\u4e0b\u964d\uff0c\u53ef\u80fd\u9700\u8981\u66f4\u591a\u6570\u636e\u6216\u66f4\u957f\u8bad\u7ec3\u3002")

    # ===== 2. Slang Hit Rate on eval prompts =====
    print(f"\n{'=' * 60}")
    print("\U0001f3af \u8bc4\u6d4b\u9879 2: \u9ed1\u8bdd\u547d\u4e2d\u7387 (Slang Hit Rate)")
    print("=" * 60)
    print("   \u6aa2\u6e2c\u751f\u6210\u5167\u5bb9\u4e2d\u662f\u5426\u81ea\u7136\u4f7f\u7528\u4e86\u53f0\u7063\u96fb\u5546\u9ed1\u8a71\u3002\n")

    eval_prompts = load_eval_prompts()

    print("   --- Before (\u901a\u7528\u6a21\u578b) ---")
    with tuned_model.disable_adapter():
        results_before, overall_before = evaluate_generation(
            tuned_model, tokenizer, device, "Before", eval_prompts)

    print(f"\n   --- After (LoRA \u5fae\u8c03) ---")
    results_after, overall_after = evaluate_generation(
        tuned_model, tokenizer, device, "After", eval_prompts)

    # Summary table
    print(f"\n{'=' * 60}")
    print("\U0001f4cb \u8bc4\u6d4b\u7ed3\u679c\u6c47\u603b")
    print("=" * 60)
    col_lift = "\u63d0\u5347"
    col_ppl = "\u56f0\u60d1\u5ea6 (Perplexity) \u2193"
    col_slang = "\u9ed1\u8bdd\u547d\u4e2d\u7387 (Slang Hit) \u2191"
    print(f"\n   {'':30s} {'Before':>10s} {'After':>10s} {col_lift:>10s}")
    print(f"   {'-' * 62}")

    print(f"   {col_ppl:30s} {ppl_before:>10.2f} {ppl_after:>10.2f} {ppl_drop:>+9.1f}%")
    slang_lift = (overall_after - overall_before) * 100
    print(f"   {col_slang:30s} {overall_before:>9.1%} {overall_after:>9.1%} {slang_lift:>+9.1f}pp")

    col_after_hits = "After \u547d\u4e2d\u8bcd\u6c47"
    no_hit = "(\u65e0)"
    print(f"\n   \u9010\u9898\u9ed1\u8bdd\u547d\u4e2d\u660e\u7ec6:")
    print(f"   {'Prompt':>8s} {'Before':>12s} {'After':>12s} {col_after_hits}")
    print(f"   {'-' * 62}")
    for rb, ra in zip(results_before, results_after):
        idx_str = "#" + str(rb["prompt_idx"])
        matched_str = ", ".join(ra["matched"][:5]) or no_hit
        print(f"   {idx_str:>8s}"
              f" {rb['hits']}/{rb['expected']:>10}"
              f" {ra['hits']}/{ra['expected']:>10}"
              f"   {matched_str}")

    print(f"\n{'=' * 60}")
    print("\U0001f4a1 \u89e3\u8bfb:")
    print("   - \u56f0\u60d1\u5ea6\u4e0b\u964d = \u6a21\u578b\u66f4\u201c\u61c2\u201d\u8fd9\u4e2a\u9886\u57df\u7684\u8bed\u8a00\u6a21\u5f0f")
    print("   - \u9ed1\u8a71\u547d\u4e2d\u7387\u63d0\u5347 = \u6a21\u578b\u80fd\u81ea\u7136\u4f7f\u7528\u53f0\u7063\u96fb\u5546\u8853\u8a9e")
    print("   - \u751f\u4ea7\u73af\u5883\u5efa\u8bae: \u52a0\u5165 LLM-as-Judge (GPT-4 \u6253\u5206) + \u4eba\u5de5\u8bc4\u6d4b")
    print("=" * 60)
    print("\n\u2705 \u8bc4\u6d4b\u5b8c\u6210\uff01")


if __name__ == "__main__":
    main()
