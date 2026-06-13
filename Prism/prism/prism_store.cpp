#include "prism_store.h"
#include <vector>
#include <cstring>
#include <cstdio>
#include <algorithm>
#include <cmath>
#include <string>   // 加上这一行

struct MemoryRecord {
    uint64_t id;
    std::string role;
    std::string content;
    std::string keywords;
    std::vector<float> embedding;
};

struct PrismStore {
    std::string data_path;
    FILE* file;                     // 追加写文件句柄
    std::vector<MemoryRecord> records; // 内存全量缓存
    uint64_t next_id = 1;

    ~PrismStore() {
        if (file) fclose(file);
    }
};

// 写入一条记录到文件末尾（二进制格式）
static void append_to_file(FILE* f, const MemoryRecord& rec) {
    // 写入 id (8字节)
    fwrite(&rec.id, sizeof(uint64_t), 1, f);
    // 写入 role 长度 + 数据
    uint32_t role_len = rec.role.size();
    fwrite(&role_len, sizeof(uint32_t), 1, f);
    fwrite(rec.role.data(), 1, role_len, f);
    // 写入 content 长度 + 数据
    uint32_t content_len = rec.content.size();
    fwrite(&content_len, sizeof(uint32_t), 1, f);
    fwrite(rec.content.data(), 1, content_len, f);
    // 写入 keywords 长度 + 数据
    uint32_t kw_len = rec.keywords.size();
    fwrite(&kw_len, sizeof(uint32_t), 1, f);
    fwrite(rec.keywords.data(), 1, kw_len, f);
    // 写入向量维度 + 向量数据
    uint32_t dim = rec.embedding.size();
    fwrite(&dim, sizeof(uint32_t), 1, f);
    fwrite(rec.embedding.data(), sizeof(float), dim, f);
    fflush(f);
}

// 从文件读一条记录，返回读取的字节数（0 表示 EOF）
static size_t read_record(FILE* f, MemoryRecord& rec) {
    uint64_t id;
    if (fread(&id, sizeof(uint64_t), 1, f) != 1) return 0;
    rec.id = id;
    
    auto read_string = [&](std::string& s) -> bool {
        uint32_t len;
        if (fread(&len, sizeof(uint32_t), 1, f) != 1) return false;
        s.resize(len);
        if (fread(&s[0], 1, len, f) != len) return false;
        return true;
    };
    
    if (!read_string(rec.role)) return 0;
    if (!read_string(rec.content)) return 0;
    if (!read_string(rec.keywords)) return 0;
    
    uint32_t dim;
    if (fread(&dim, sizeof(uint32_t), 1, f) != 1) return 0;
    rec.embedding.resize(dim);
    if (fread(rec.embedding.data(), sizeof(float), dim, f) != dim) return 0;
    
    return 1;
}

extern "C" PrismStore* prism_open(const char* data_path) {
    auto* store = new PrismStore();
    store->data_path = data_path;
    
    // 打开文件用于追加读/写
    store->file = fopen(data_path, "a+b"); // 二进制追加+读
    if (!store->file) {
        delete store;
        return nullptr;
    }
    
    // 读取已有数据到内存
    fseek(store->file, 0, SEEK_SET);
    MemoryRecord rec;
    while (read_record(store->file, rec)) {
        store->records.push_back(std::move(rec));
        if (rec.id >= store->next_id) {
            store->next_id = rec.id + 1;
        }
    }
    // 将文件指针移到末尾，准备追加
    fseek(store->file, 0, SEEK_END);
    
    return store;
}

extern "C" void prism_close(PrismStore* store) {
    delete store;
}

extern "C" uint64_t prism_insert(
    PrismStore* store,
    const char* role,
    const char* content,
    const char* keywords,
    const float* embedding,
    int embedding_dim)
{
    MemoryRecord rec;
    rec.id = store->next_id++;
    rec.role = role;
    rec.content = content;
    rec.keywords = keywords ? keywords : "";
    rec.embedding.assign(embedding, embedding + embedding_dim);
    
    append_to_file(store->file, rec);
    store->records.push_back(std::move(rec));
    return rec.id;
}

extern "C" int prism_search(
    PrismStore* store,
    const float* query_vec,
    int dim,
    int top_k,
    PrismResult* results,
    float min_score)
{
    struct Candidate {
        uint64_t id;
        float score;
        int idx; // 在 store->records 中的索引
    };
    std::vector<Candidate> heap;
    heap.reserve(top_k);
    
    const auto& records = store->records;
    for (int i = 0; i < (int)records.size(); ++i) {
        const auto& rec = records[i];
        if (rec.embedding.empty() || (int)rec.embedding.size() != dim) continue;
        
        // 点积计算（假设向量已归一化，直接得余弦相似度）
        float dot = 0.0f;
        for (int j = 0; j < dim; ++j) {
            dot += query_vec[j] * rec.embedding[j];
        }
        if (dot < min_score) continue;
        
        heap.push_back({rec.id, dot, i});
        std::push_heap(heap.begin(), heap.end(), [](const Candidate& a, const Candidate& b) {
            return a.score > b.score; // 小顶堆
        });
        if ((int)heap.size() > top_k) {
            std::pop_heap(heap.begin(), heap.end(), [](const Candidate& a, const Candidate& b) {
                return a.score > b.score;
            });
            heap.pop_back();
        }
    }
    
    std::sort(heap.begin(), heap.end(), [](const Candidate& a, const Candidate& b) {
        return a.score > b.score;
    });
    
    int count = std::min(top_k, (int)heap.size());
    for (int i = 0; i < count; ++i) {
        const auto& rec = records[heap[i].idx];
        results[i].id = rec.id;
        results[i].score = heap[i].score;
        // 分配字符串（调用者用 prism_free_results 释放）
        results[i].role = strdup(rec.role.c_str());
        results[i].content = strdup(rec.content.c_str());
    }
    return count;
}

extern "C" void prism_free_results(PrismResult* results, int count) {
    for (int i = 0; i < count; ++i) {
        free(results[i].role);
        free(results[i].content);
    }
}

extern "C" int prism_count(PrismStore* store) {
    return store->records.size();
}