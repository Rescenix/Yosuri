from tokenizers import Tokenizer
from onnxruntime import InferenceSession
import numpy as np

tokenizer = Tokenizer.from_file("tokenizer.json")
session = InferenceSession("bge-large-zh.onnx")

def embed(text):
    encoded = tokenizer.encode(text)
    input_ids = np.array([encoded.ids], dtype=np.int64)
    attention_mask = np.array([encoded.attention_mask], dtype=np.int64)
    token_type_ids = np.zeros_like(input_ids)
    ort_inputs = {
        "input_ids": input_ids,
        "attention_mask": attention_mask,
        "token_type_ids": token_type_ids
    }
    out = session.run(["last_hidden_state"], ort_inputs)[0]
    vec = out.mean(axis=1)
    vec = vec / np.linalg.norm(vec, axis=1, keepdims=True)
    return vec[0].astype(np.float32)

# ----- 记忆库 -----
memories = [
    ("user", "你好，今天天气真好"),
    ("assistant", "我喜欢机器学习和混沌理论"),
    ("user", "忆阻器是一种新型存储元件"),
    ("user", "时序记忆引擎的设计需要考虑混沌动力学"),
    ("assistant", "C++17 和 Rust 都是高性能语言"),
]

# 写入记忆向量文件 (每1024维一个float32数组)
with open("memories.bin", "wb") as f:
    for _, text in memories:
        f.write(embed(text).tobytes())

# ----- 预定义查询向量 -----
queries = {
    "机器学习": "机器学习",
    "天气": "天气",
    "忆阻器": "忆阻器",
    "混沌": "混沌",
    "编程语言": "编程语言",
    "你好": "你好",
}

for name, text in queries.items():
    vec = embed(text)
    with open(f"query_{name}.bin", "wb") as f:
        f.write(vec.tobytes())

# 同时保存记忆文本元数据供C++使用
with open("memories_meta.txt", "w", encoding="utf-8") as f:
    for role, text in memories:
        f.write(f"{role}|{text}\n")

print("所有演示素材生成完毕。")