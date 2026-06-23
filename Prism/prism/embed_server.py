from http.server import HTTPServer, BaseHTTPRequestHandler
from tokenizers import Tokenizer
from onnxruntime import InferenceSession
import numpy as np

tokenizer = Tokenizer.from_file("tokenizer.json")
session = InferenceSession("bge-large-zh.onnx")
MAX_LEN = 512

def embed(text):
    # 编码（不传 max_length 参数，避免库版本问题）
    encoded = tokenizer.encode(text)
    ids = encoded.ids
    # 手动截断到 MAX_LEN
    if len(ids) > MAX_LEN:
        ids = ids[:MAX_LEN]
    input_ids = np.array([ids], dtype=np.int64)
    attention_mask = np.ones_like(input_ids)
    token_type_ids = np.zeros_like(input_ids)
    out = session.run(["last_hidden_state"], {
        "input_ids": input_ids,
        "attention_mask": attention_mask,
        "token_type_ids": token_type_ids
    })[0]
    vec = out.mean(axis=1)
    vec = vec / np.linalg.norm(vec, axis=1, keepdims=True)
    return vec[0].astype(np.float32)

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers.get('Content-Length', 0))
        raw_body = self.rfile.read(content_length)
        text = raw_body.decode('utf-8', errors='ignore').strip()
        if not text:
            self.send_response(400)
            self.end_headers()
            return
        try:
            vec = embed(text)
            self.send_response(200)
            self.send_header("Content-Type", "application/octet-stream")
            self.end_headers()
            self.wfile.write(vec.tobytes())
        except Exception as e:
            print(f"Embedding error: {e}")
            self.send_response(500)
            self.end_headers()

print("⚡ BGE Embedding Server 已启动: http://localhost:6752")
HTTPServer(("", 6752), Handler).serve_forever()