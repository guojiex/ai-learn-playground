"""Step 3: Training — 使用东南亚电商数据微调模型。

用 JSONL 样本数据跑少量训练步骤，展示 loss 下降过程，
并将训练好的 LoRA adapter 保存到 output/ 目录。

收敛策略:
  - 设置最大 epoch 上限（MAX_EPOCHS），作为保底
  - Early stopping: 连续 PATIENCE 个 epoch loss 下降不超过
    MIN_DELTA 时自动停止，避免过拟合和浪费时间
  - 始终保存 best model（loss 最低的那个 epoch）

断点续训:
  - 每个 epoch 结束后保存 checkpoint（含 best_loss / patience 状态）
  - 下次运行时自动从最近的 checkpoint 恢复

性能优化:
  - 动态 padding（按 batch 内最长序列对齐，而非固定 256）
  - batch_size=4 + gradient_accumulation=4 → 有效 batch=16
  - MPS/CUDA 自动混合精度
  - 日志精简（每 10 步打印一次）
"""

import json
import os
import shutil
import time
import torch
from pathlib import Path
from torch.utils.data import Dataset, DataLoader
from peft import get_peft_model, PeftModel, TaskType
from transformers import default_data_collator

from utils import detect_device, load_base_model, ADAPTER_DIR, DATA_PATH
from step2_lora_inject import get_lora_config

CHECKPOINT_DIR = ADAPTER_DIR / "checkpoints"
BEST_DIR = ADAPTER_DIR / "best"

BATCH_SIZE = 4
GRAD_ACCUM_STEPS = 4
LOG_EVERY = 10

MAX_EPOCHS = 30
PATIENCE = 3
MIN_DELTA = 0.01


class AffiliateDataset(Dataset):
    """Simple dataset that tokenizes JSONL instruction-output pairs.

    Does NOT pad here — padding is deferred to the collator so each batch
    only pads to its own longest sequence instead of a fixed max_length.
    """

    def __init__(self, path, tokenizer, max_length=256):
        self.samples = []
        with open(path, "r", encoding="utf-8") as f:
            for line in f:
                item = json.loads(line.strip())
                instruction = item["instruction"]
                inp = item.get("input", "")
                output = item["output"]

                if inp:
                    user_msg = f"{instruction}\n{inp}"
                else:
                    user_msg = instruction

                messages = [
                    {"role": "user", "content": user_msg},
                    {"role": "assistant", "content": output},
                ]
                text = tokenizer.apply_chat_template(messages, tokenize=False)
                encoded = tokenizer(
                    text,
                    truncation=True,
                    max_length=max_length,
                    return_tensors="pt",
                )
                input_ids = encoded["input_ids"].squeeze(0)
                attention_mask = encoded["attention_mask"].squeeze(0)
                self.samples.append({
                    "input_ids": input_ids,
                    "attention_mask": attention_mask,
                    "labels": input_ids.clone(),
                })

    def __len__(self):
        return len(self.samples)

    def __getitem__(self, idx):
        return self.samples[idx]


def dynamic_pad_collator(batch, pad_token_id=0):
    """Pad to the longest sequence in the batch instead of a fixed length."""
    max_len = max(item["input_ids"].size(0) for item in batch)
    input_ids, attention_mask, labels = [], [], []
    for item in batch:
        seq_len = item["input_ids"].size(0)
        pad_len = max_len - seq_len
        input_ids.append(torch.cat([item["input_ids"], torch.full((pad_len,), pad_token_id)]))
        attention_mask.append(torch.cat([item["attention_mask"], torch.zeros(pad_len, dtype=torch.long)]))
        labels.append(torch.cat([item["labels"], torch.full((pad_len,), -100)]))
    return {
        "input_ids": torch.stack(input_ids),
        "attention_mask": torch.stack(attention_mask),
        "labels": torch.stack(labels),
    }


def find_latest_checkpoint():
    """Find the latest checkpoint directory by epoch number."""
    if not CHECKPOINT_DIR.exists():
        return None, 0
    ckpts = sorted(CHECKPOINT_DIR.glob("epoch-*"), key=lambda p: int(p.name.split("-")[1]))
    if not ckpts:
        return None, 0
    latest = ckpts[-1]
    epoch_num = int(latest.name.split("-")[1])
    return latest, epoch_num


def save_checkpoint(model, optimizer, epoch, global_step, best_loss, patience_counter):
    """Save new checkpoint and delete the previous one (only keep latest)."""
    # Delete all old checkpoints first
    if CHECKPOINT_DIR.exists():
        for old in CHECKPOINT_DIR.glob("epoch-*"):
            if old.is_dir():
                shutil.rmtree(old)

    ckpt_path = CHECKPOINT_DIR / f"epoch-{epoch}"
    ckpt_path.mkdir(parents=True, exist_ok=True)
    model.save_pretrained(str(ckpt_path))
    torch.save({
        "optimizer_state_dict": optimizer.state_dict(),
        "epoch": epoch,
        "global_step": global_step,
        "best_loss": best_loss,
        "patience_counter": patience_counter,
    }, ckpt_path / "training_state.pt")
    print(f"   💾 Checkpoint 已保存: {ckpt_path.name} (已清理旧 checkpoint)")


def save_best_model(model, tokenizer):
    """Overwrite the best model snapshot whenever a new best loss is reached."""
    BEST_DIR.mkdir(parents=True, exist_ok=True)
    model.save_pretrained(str(BEST_DIR))
    tokenizer.save_pretrained(str(BEST_DIR))
    print(f"   ⭐ Best model 已更新: {BEST_DIR}")


def main():
    print("=" * 60)
    print('\U0001f4cc Step 3: \u5fae\u8c03\u8bad\u7ec3 \u2014 \u7528 5% \u4e1a\u52a1\u6570\u636e\u6ce8\u5165\u201c\u672c\u5730\u7075\u9b42\u201d')
    print("=" * 60)

    learning_rate = 3e-4

    device, use_quant = detect_device()
    tokenizer, model = load_base_model(device, use_quant)

    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token
    pad_token_id = tokenizer.pad_token_id

    if use_quant:
        from peft import prepare_model_for_kbit_training
        model = prepare_model_for_kbit_training(model)

    latest_ckpt, start_epoch = find_latest_checkpoint()
    best_loss = float("inf")
    patience_counter = 0
    already_converged = False

    if latest_ckpt is not None:
        training_state = torch.load(latest_ckpt / "training_state.pt", map_location=device, weights_only=True)
        best_loss = training_state.get("best_loss", float("inf"))
        patience_counter = training_state.get("patience_counter", 0)
        global_step = training_state["global_step"]

        if patience_counter >= PATIENCE:
            print(f"\n✅ 之前已在 epoch {start_epoch} 收敛停止 (patience={PATIENCE})，无需继续训练。")
            already_converged = True
        elif start_epoch >= MAX_EPOCHS:
            print(f"\n✅ 已达到最大 epoch 上限 ({MAX_EPOCHS})，无需继续训练。")
            already_converged = True
        else:
            print(f"\n♻️  检测到 checkpoint: {latest_ckpt.name}，从 epoch {start_epoch + 1} 继续训练")
            print(f"   best_loss={best_loss:.4f}, patience={patience_counter}/{PATIENCE}")

        model = PeftModel.from_pretrained(model, str(latest_ckpt))
    else:
        print("\n🆕 未检测到 checkpoint，从头开始训练")
        peft_config = get_lora_config()
        model = get_peft_model(model, peft_config)
        training_state = None
        global_step = 0

    model.print_trainable_parameters()

    print(f"\n📂 加载训练数据: {DATA_PATH}")
    dataset = AffiliateDataset(DATA_PATH, tokenizer)
    print(f"   样本数量: {len(dataset)}")

    collator = lambda batch: dynamic_pad_collator(batch, pad_token_id)
    dataloader = DataLoader(dataset, batch_size=BATCH_SIZE, shuffle=True, collate_fn=collator)

    effective_batch = BATCH_SIZE * GRAD_ACCUM_STEPS
    steps_per_epoch = len(dataloader)

    optimizer = torch.optim.AdamW(model.parameters(), lr=learning_rate)
    if training_state is not None and not already_converged:
        optimizer.load_state_dict(training_state["optimizer_state_dict"])
        print(f"   ♻️  已恢复 optimizer 状态 (global_step={global_step})")

    use_amp = device.type in ("cuda", "mps")
    amp_dtype = torch.float16 if device.type == "cuda" else torch.bfloat16
    scaler = torch.amp.GradScaler(device.type) if device.type == "cuda" else None

    model.train()

    if not already_converged:
        print(f"\n🚀 开始训练 (最大 {MAX_EPOCHS} epochs, early stopping patience={PATIENCE}, min_delta={MIN_DELTA})")
        print(f"   batch_size={BATCH_SIZE} x grad_accum={GRAD_ACCUM_STEPS} → 有效 batch={effective_batch}")
        print(f"   每 epoch {steps_per_epoch} 步")
        print("-" * 50)

        t0 = time.time()
        for epoch in range(start_epoch + 1, MAX_EPOCHS + 1):
            epoch_loss = 0.0
            epoch_steps = 0
            optimizer.zero_grad()

            for batch_idx, batch in enumerate(dataloader):
                batch = {k: v.to(device) for k, v in batch.items()}

                if use_amp and device.type == "cuda":
                    with torch.amp.autocast(device.type, dtype=amp_dtype):
                        outputs = model(**batch)
                        loss = outputs.loss / GRAD_ACCUM_STEPS
                    scaler.scale(loss).backward()
                else:
                    outputs = model(**batch)
                    loss = outputs.loss / GRAD_ACCUM_STEPS
                    loss.backward()

                epoch_loss += loss.item() * GRAD_ACCUM_STEPS
                global_step += 1
                epoch_steps += 1

                is_accum_step = (batch_idx + 1) % GRAD_ACCUM_STEPS == 0 or (batch_idx + 1) == len(dataloader)
                if is_accum_step:
                    if scaler is not None:
                        scaler.step(optimizer)
                        scaler.update()
                    else:
                        optimizer.step()
                    optimizer.zero_grad()

                if global_step % LOG_EVERY == 0:
                    elapsed = time.time() - t0
                    avg = epoch_loss / epoch_steps
                    print(f"   Step {global_step:4d} | Loss: {loss.item() * GRAD_ACCUM_STEPS:.4f} | "
                          f"Avg: {avg:.4f} | {elapsed:.0f}s")

            avg_loss = epoch_loss / epoch_steps
            elapsed = time.time() - t0

            improvement = best_loss - avg_loss
            if improvement > MIN_DELTA:
                best_loss = avg_loss
                patience_counter = 0
                save_best_model(model, tokenizer)
                status = "⬇️  new best"
            else:
                patience_counter += 1
                status = f"⏸️  no improve ({patience_counter}/{PATIENCE})"

            print(f"   --- Epoch {epoch} | Avg Loss: {avg_loss:.4f} | Best: {best_loss:.4f} | "
                  f"{status} | {elapsed:.0f}s ---")
            save_checkpoint(model, optimizer, epoch, global_step, best_loss, patience_counter)

            if patience_counter >= PATIENCE:
                print(f"\n   🛑 Early stopping: 连续 {PATIENCE} 个 epoch loss 未明显下降 (delta < {MIN_DELTA})")
                print(f"   最终停在 epoch {epoch}，best loss = {best_loss:.4f}")
                break

        total_time = time.time() - t0
        print(f"\n   ⏱️  训练总耗时: {total_time:.0f}s ({total_time / 60:.1f} min)")
        print("-" * 50)

    ADAPTER_DIR.mkdir(parents=True, exist_ok=True)
    if BEST_DIR.exists():
        for f in BEST_DIR.iterdir():
            if f.is_file():
                shutil.copy2(f, ADAPTER_DIR / f.name)
        print(f"\n💾 最佳 LoRA 适配器已保存到: {ADAPTER_DIR} (来自 best model)")
    else:
        model.save_pretrained(str(ADAPTER_DIR))
        tokenizer.save_pretrained(str(ADAPTER_DIR))
        print(f"\n💾 LoRA 适配器已保存到: {ADAPTER_DIR}")

    # 清理: 删除 checkpoints 和 best 临时目录
    for tmp_dir in (CHECKPOINT_DIR, BEST_DIR):
        if tmp_dir.exists():
            shutil.rmtree(tmp_dir)
    print("   🧹 已清理 checkpoints 和 best 临时目录")

    adapter_size = sum(
        os.path.getsize(ADAPTER_DIR / f)
        for f in os.listdir(ADAPTER_DIR)
        if os.path.isfile(ADAPTER_DIR / f)
    )
    print(f"   适配器文件大小: {adapter_size / 1024 / 1024:.1f} MB（对比底座模型数 GB）")
    print("\n✅ 训练完成！下一步: 加载适配器进行推理对比。")


if __name__ == "__main__":
    main()
