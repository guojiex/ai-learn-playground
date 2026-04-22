#!/usr/bin/env python3
"""Debug script: download + load model with full progress visibility."""
import os
import sys
import time

MODEL_ID = os.getenv("AFFILIATE_MODEL_PATH", "Qwen/Qwen2.5-0.5B-Instruct")

def step(msg: str):
    print(f"\n{'='*60}")
    print(f"  {msg}")
    print(f"{'='*60}")
    return time.time()

# ── Step 1: check cache ──────────────────────────────────────
t = step(f"Checking HuggingFace cache for: {MODEL_ID}")
from huggingface_hub import scan_cache_dir, snapshot_download

cache = scan_cache_dir()
cached = [r for r in cache.repos if r.repo_id == MODEL_ID]
if cached:
    sizes = [f"{r.size_on_disk / 1e6:.0f} MB" for r in cached]
    print(f"  Already cached: {sizes}")
else:
    print(f"  Not in cache yet — will download")
print(f"  ({time.time() - t:.1f}s)")

# ── Step 2: download (with progress bar) ─────────────────────
t = step(f"Downloading / verifying: {MODEL_ID}")
local_dir = snapshot_download(MODEL_ID)
print(f"  Local path: {local_dir}")
print(f"  ({time.time() - t:.1f}s)")

# ── Step 3: load tokenizer ───────────────────────────────────
t = step("Loading tokenizer")
from transformers import AutoTokenizer
tokenizer = AutoTokenizer.from_pretrained(MODEL_ID, trust_remote_code=True)
print(f"  Vocab size: {tokenizer.vocab_size}")
print(f"  ({time.time() - t:.1f}s)")

# ── Step 4: load model ───────────────────────────────────────
t = step("Loading model weights")
import torch
from transformers import AutoModelForCausalLM

device_label = "cpu"
load_kw = dict(trust_remote_code=True, low_cpu_mem_usage=True)

if torch.cuda.is_available():
    load_kw["device_map"] = "auto"
    device = torch.device("cuda")
    device_label = f"cuda ({torch.cuda.get_device_name(0)})"
elif torch.backends.mps.is_available():
    load_kw["torch_dtype"] = torch.bfloat16
    device = torch.device("mps")
    device_label = "mps (Apple Silicon)"
else:
    load_kw["torch_dtype"] = torch.bfloat16
    device = torch.device("cpu")

print(f"  Target device: {device_label}")
model = AutoModelForCausalLM.from_pretrained(MODEL_ID, **load_kw)
if "device_map" not in load_kw:
    model = model.to(device)
model.eval()

param_count = sum(p.numel() for p in model.parameters()) / 1e6
print(f"  Parameters: {param_count:.0f}M")
print(f"  ({time.time() - t:.1f}s)")

# ── Step 5: test inference ────────────────────────────────────
t = step("Test inference")
messages = [
    {"role": "system", "content": "Reply with a single JSON object: {\"status\": \"ok\"}"},
    {"role": "user", "content": "ping"},
]
text = tokenizer.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
inputs = tokenizer(text, return_tensors="pt").to(device)
with torch.no_grad():
    out = model.generate(**inputs, max_new_tokens=64, temperature=0.7, do_sample=True)
reply = tokenizer.decode(out[0][inputs["input_ids"].shape[1]:], skip_special_tokens=True).strip()
print(f"  Model reply: {reply}")
print(f"  ({time.time() - t:.1f}s)")

print(f"\n{'='*60}")
print("  All done! Model is working correctly.")
print(f"{'='*60}\n")
