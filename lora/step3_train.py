"""Step 3: Training — 使用东南亚电商数据微调模型。

用 JSONL 样本数据跑少量训练步骤，展示 loss 下降过程，
并将训练好的 LoRA adapter 保存到 output/ 目录。
"""

import json
import torch
from torch.utils.data import Dataset, DataLoader
from peft import get_peft_model, TaskType
from transformers import default_data_collator

from utils import detect_device, load_base_model, ADAPTER_DIR, DATA_PATH
from step2_lora_inject import get_lora_config


class AffiliateDataset(Dataset):
    """Simple dataset that tokenizes JSONL instruction-output pairs."""

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
                    padding="max_length",
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


def main():
    print("=" * 60)
    print('\U0001f4cc Step 3: \u5fae\u8c03\u8bad\u7ec3 \u2014 \u7528 5% \u4e1a\u52a1\u6570\u636e\u6ce8\u5165\u201c\u672c\u5730\u7075\u9b42\u201d')
    print("=" * 60)

    num_epochs = 10
    learning_rate = 3e-4

    device, use_quant = detect_device()
    tokenizer, model = load_base_model(device, use_quant)

    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    if use_quant:
        from peft import prepare_model_for_kbit_training
        model = prepare_model_for_kbit_training(model)

    peft_config = get_lora_config()
    model = get_peft_model(model, peft_config)
    model.print_trainable_parameters()

    print(f"\n📂 加载训练数据: {DATA_PATH}")
    dataset = AffiliateDataset(DATA_PATH, tokenizer)
    print(f"   样本数量: {len(dataset)}")
    dataloader = DataLoader(dataset, batch_size=1, shuffle=True, collate_fn=default_data_collator)

    optimizer = torch.optim.AdamW(model.parameters(), lr=learning_rate)
    model.train()

    print(f"\n🚀 开始训练 (epochs={num_epochs}, lr={learning_rate})")
    print("-" * 50)

    global_step = 0
    for epoch in range(num_epochs):
        epoch_loss = 0.0
        for batch in dataloader:
            batch = {k: v.to(device) for k, v in batch.items()}
            outputs = model(**batch)
            loss = outputs.loss
            loss.backward()
            optimizer.step()
            optimizer.zero_grad()

            epoch_loss += loss.item()
            global_step += 1
            print(f"   Step {global_step:3d} | Loss: {loss.item():.4f}")

        avg_loss = epoch_loss / len(dataloader)
        print(f"   --- Epoch {epoch + 1}/{num_epochs} 完成 | 平均 Loss: {avg_loss:.4f} ---")

    print("-" * 50)

    ADAPTER_DIR.mkdir(parents=True, exist_ok=True)
    model.save_pretrained(str(ADAPTER_DIR))
    tokenizer.save_pretrained(str(ADAPTER_DIR))
    print(f"\n💾 LoRA 适配器已保存到: {ADAPTER_DIR}")

    import os
    adapter_size = sum(
        os.path.getsize(ADAPTER_DIR / f)
        for f in os.listdir(ADAPTER_DIR)
        if os.path.isfile(ADAPTER_DIR / f)
    )
    print(f"   适配器文件大小: {adapter_size / 1024 / 1024:.1f} MB（对比底座模型数 GB）")
    print("\n✅ 训练完成！下一步: 加载适配器进行推理对比。")


if __name__ == "__main__":
    main()
