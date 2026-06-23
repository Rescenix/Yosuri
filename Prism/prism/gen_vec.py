from tokenizers import Tokenizer
from onnxruntime import InferenceSession
import numpy as np

# 加载本地 tokenizer.json 和 onnx 模型
tokenizer = Tokenizer.from_file("tokenizer.json")
session = InferenceSession("bge-large-zh.onnx")

def embed(text):
    encoded = tokenizer.encode(text)
    input_ids = np.array([encoded.ids], dtype=np.int64)
    attention_mask = np.array([encoded.attention_mask], dtype=np.int64)
    # BGE 模型还需要 token_type_ids，通常全为 0
    token_type_ids = np.zeros_like(input_ids)

    ort_inputs = {
        "input_ids": input_ids,
        "attention_mask": attention_mask,
        "token_type_ids": token_type_ids
    }
    out = session.run(["last_hidden_state"], ort_inputs)[0]
    # 平均池化 + 归一化
    vec = out.mean(axis=1)
    vec = vec / np.linalg.norm(vec, axis=1, keepdims=True)
    return vec[0].astype(np.float32)

texts = [
    "你好，今天天气真好",
    "我喜欢机器学习和混沌理论",
    "忆阻器是一种新型存储元件"
]
all_vecs = [embed(t) for t in texts]

with open("vectors.bin", "wb") as f:
    for v in all_vecs:
        f.write(v.tobytes())

print("向量已保存到 vectors.bin")

# 生成查询向量
query_text = "机器学习"
qvec = embed(query_text)
with open("query.bin", "wb") as f:
    f.write(qvec.tobytes())
print(f"查询向量已保存到 query.bin")