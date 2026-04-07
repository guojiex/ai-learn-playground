import torch
import os
from transformers import AutoModelForCausalLM, AutoTokenizer
from peft import LoraConfig, get_peft_model, TaskType

def main():
    # 1. 基础配置
    model_id = "Qwen/Qwen1.5-1.8B-Chat" # 适合演示的轻量级模型
    
    # 检测设备：M1 Pro 优先使用 mps (Metal Performance Shaders)
    if torch.backends.mps.is_available():
        device = torch.device("mps")
        print("🍏 检测到 Apple Silicon，使用 MPS 加速")
    else:
        device = torch.device("cpu")
        print("💻 未检测到 GPU 加速，使用 CPU")

    # 2. 加载分词器和模型
    print(f"📦 正在加载模型: {model_id}...")
    tokenizer = AutoTokenizer.from_pretrained(model_id, trust_remote_code=True)
    
    # 在 Mac 上，我们使用 float16 来兼顾速度和显存
    model = AutoModelForCausalLM.from_pretrained(
        model_id,
        dtype=torch.bfloat16,
        low_cpu_mem_usage=True,
        trust_remote_code=True
    ).to(device)

    # 3. 定义 LoRA 配置 (针对东南亚电商推客场景)
    print("🛠️ 正在注入 LoRA 适配器 (微调贴纸)...")
    peft_config = LoraConfig(
        task_type=TaskType.CAUSAL_LM,
        inference_mode=False, 
        r=16,                # 秩：决定了微调的参数量
        lora_alpha=32,       # 缩放系数
        lora_dropout=0.1,
        # 针对 Qwen 模型的注意力机制层进行微调
        target_modules=["q_proj", "v_proj", "k_proj", "o_proj"] 
    )

    # 4. 包装模型
    model = get_peft_model(model, peft_config)
    
    # 展示核心卖点：可训练参数占比
    model.print_trainable_parameters()

    # 5. 模拟推理演示：生成东南亚电商文案
    print("\n✨ 正在模拟推客文案生成...")
    # 模拟一个印尼市场的种草场景
    prompt = "你是印尼 TikTok 顶级推客。请用本地黑话安利这款充电宝: Powerbank 20000mAh, 100k IDR."
    
    # 构造 Prompt 模板 (针对 Qwen)
    inputs = tokenizer(f"<|im_start|>user\n{prompt}<|im_end|>\n<|im_start|>assistant\n", return_tensors="pt").to(device)

    # 生成预测
    with torch.no_grad():
        output_tokens = model.generate(
            **inputs,
            max_new_tokens=150,
            temperature=0.7,
            top_p=0.9
        )

    # 解码并打印结果
    result = tokenizer.decode(output_tokens[0], skip_special_tokens=True)
    print("-" * 50)
    print(f"生成的文案内容:\n\n{result.split('assistant')[-1].strip()}")
    print("-" * 50)
    print("\n✅ 演示运行成功！")

if __name__ == "__main__":
    main()