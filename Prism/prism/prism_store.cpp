#include "prism_store.h"
#include <vector>
#include <cstring>
#include <cstdio>
#include <algorithm>
#include <chrono>
#include <string>
#include <unordered_map>
#include <sstream>
#include <cmath>

// ============ 极简中文分词与 TF‑IDF 引擎 ============
static std::vector<std::string> tokenize(const std::string& text) {
    std::vector<std::string> tokens;
    // 先按单字切分
    std::vector<std::string> chars;
    for (size_t i = 0; i < text.size(); ) {
        unsigned char c = text[i];
        size_t len = 1;
        if (c >= 0xE0 && c < 0xF0) len = 3;
        else if (c >= 0xC0 && c < 0xE0) len = 2;
        else if (c >= 0xF0) len = 4;
        chars.push_back(text.substr(i, len));
        i += len;
    }
    // 生成 bigram
    for (size_t i = 0; i + 1 < chars.size(); i++) {
        tokens.push_back(chars[i] + chars[i+1]);
    }
    return tokens;
}

static std::unordered_map<std::string, int> g_vocab;
static std::vector<int> g_doc_count;

static int register_token(const std::string& token) {
    auto it = g_vocab.find(token);
    if (it != g_vocab.end()) return it->second;
    int idx = (int)g_vocab.size();
    g_vocab[token] = idx;
    g_doc_count.push_back(0);
    return idx;
}

static std::vector<float> text_to_vector(const std::string& text) {
    auto tokens = tokenize(text);
    std::unordered_map<int, float> tf;
    for (const auto& t : tokens) {
        auto it = g_vocab.find(t);
        if (it == g_vocab.end()) continue;
        tf[it->second] += 1.0f;
    }

    int n = (int)g_vocab.size();
    std::vector<float> vec(n, 0.0f);
    for (const auto& p : tf) {
        int idx = p.first;
        float idf = std::log((float)g_doc_count.size() / (1.0f + g_doc_count[idx]));
        vec[idx] = p.second * idf;
    }

    float norm = 0.0f;
    for (float v : vec) norm += v * v;
    if (norm > 0) {
        norm = std::sqrt(norm);
        for (float& v : vec) v /= norm;
    }
    return vec;
}

// ============ 快速 exp ============
inline float fast_exp(float x) {
    if (x < -20.0f) return 0.0f;
    if (x > 20.0f)  return 4.85e8f;
    float x2 = x * x, x3 = x2 * x, x4 = x3 * x, x5 = x4 * x;
    return 1.0f + x + x2 * 0.5f + x3 * 0.16666667f + x4 * 0.04166667f + x5 * 0.008333334f;
}

// ============ 内部结构 ============
struct MemristorState {
    float conductance;
    float correlation_flux;
    uint64_t last_update_ts;
};

struct MemoryRecord {
    uint64_t id;
    std::string role;
    std::string content;
    std::string keywords;
    std::vector<float> embedding;
    MemristorState state;
};

struct PrismStore {
    std::string data_path;
    FILE* file;
    std::vector<MemoryRecord> records;
    uint64_t next_id = 1;
    ~PrismStore() { if (file) fclose(file); }
};

const float TAU_FORGET_MS = 3600.0f * 1000.0f;
const float INERTIA = 0.001f;
const float QUANTUM_NOISE = 1e-9f;
static bool g_evolution_enabled = true;

static uint64_t now_ms() {
    return std::chrono::duration_cast<std::chrono::milliseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();
}

// ============ 文件读写 ============
static void append_to_file(FILE* f, const MemoryRecord& rec) {
    fwrite(&rec.id, sizeof(uint64_t), 1, f);
    uint32_t role_len = rec.role.size();
    fwrite(&role_len, sizeof(uint32_t), 1, f);
    fwrite(rec.role.data(), 1, role_len, f);
    uint32_t content_len = rec.content.size();
    fwrite(&content_len, sizeof(uint32_t), 1, f);
    fwrite(rec.content.data(), 1, content_len, f);
    uint32_t kw_len = rec.keywords.size();
    fwrite(&kw_len, sizeof(uint32_t), 1, f);
    fwrite(rec.keywords.data(), 1, kw_len, f);
    uint32_t dim = rec.embedding.size();
    fwrite(&dim, sizeof(uint32_t), 1, f);
    fwrite(rec.embedding.data(), sizeof(float), dim, f);
    fwrite(&rec.state.conductance, sizeof(float), 1, f);
    fwrite(&rec.state.correlation_flux, sizeof(float), 1, f);
    fwrite(&rec.state.last_update_ts, sizeof(uint64_t), 1, f);
    fflush(f);
}

static size_t read_record(FILE* f, MemoryRecord& rec) {
    uint64_t id;
    if (fread(&id, sizeof(uint64_t), 1, f) != 1) return 0;
    rec.id = id;

    auto read_string = [&](std::string& s) -> bool {
        uint32_t len;
        if (fread(&len, sizeof(uint32_t), 1, f) != 1) return false;
        s.resize(len);
        return fread(&s[0], 1, len, f) == len;
    };

    if (!read_string(rec.role) || !read_string(rec.content) || !read_string(rec.keywords))
        return 0;

    uint32_t dim;
    if (fread(&dim, sizeof(uint32_t), 1, f) != 1) return 0;
    rec.embedding.resize(dim);
    if (fread(rec.embedding.data(), sizeof(float), dim, f) != dim) return 0;

    if (fread(&rec.state.conductance, sizeof(float), 1, f) != 1) {
        rec.state.conductance = 0.5f;
        rec.state.correlation_flux = 0.0f;
        rec.state.last_update_ts = now_ms();
        return 1;
    }
    fread(&rec.state.correlation_flux, sizeof(float), 1, f);
    fread(&rec.state.last_update_ts, sizeof(uint64_t), 1, f);
    return 1;
}

// ============ API 实现 ============
extern "C" PrismStore* prism_open(const char* data_path) {
    auto* store = new PrismStore();
    store->data_path = data_path;
    store->file = fopen(data_path, "a+b");
    if (!store->file) { delete store; return nullptr; }

    fseek(store->file, 0, SEEK_SET);
    MemoryRecord rec;
    while (read_record(store->file, rec)) {
        // 从文件恢复时，重建词库
        auto tokens = tokenize(rec.content);
        for (const auto& t : tokens) {
            int idx = register_token(t);
            g_doc_count[idx] += 1;
        }
        store->records.push_back(std::move(rec));
        if (rec.id >= store->next_id) store->next_id = rec.id + 1;
    }
    // 恢复后重新生成所有向量，保证维度一致
    for (auto& rec : store->records) {
        rec.embedding = text_to_vector(rec.content);
    }
    fseek(store->file, 0, SEEK_END);
    return store;
}

extern "C" void prism_close(PrismStore* store) { delete store; }

extern "C" uint64_t prism_insert(
    PrismStore* store, const char* role, const char* content,
    const char* keywords, const float* embedding, int embedding_dim)
{
    MemoryRecord rec;
    rec.id = store->next_id++;
    rec.role = role;
    rec.content = content;
    rec.keywords = keywords ? keywords : "";

    // 更新全局词库
    auto tokens = tokenize(content);
    for (const auto& t : tokens) {
        int idx = register_token(t);
        g_doc_count[idx] += 1;
    }

    // 重新生成所有已有记录的向量，保证维度一致
    for (auto& old_rec : store->records) {
        old_rec.embedding = text_to_vector(old_rec.content);
    }
    rec.embedding = text_to_vector(content);

    rec.state.conductance = 0.5f;
    rec.state.correlation_flux = 0.0f;
    rec.state.last_update_ts = now_ms();

    append_to_file(store->file, rec);
    store->records.push_back(std::move(rec));
    return rec.id;
}

extern "C" int prism_search_raw(
    PrismStore* store, const float* query_vec, int dim, int top_k,
    uint64_t* out_ids, float* out_scores)
{
    struct RawScore { uint64_t id; float score; };
    RawScore best[10];
    int count = 0;

    for (auto& rec : store->records) {
        if (rec.embedding.empty() || (int)rec.embedding.size() != dim) continue;
        float raw = 0.0f;
        for (int j = 0; j < dim; ++j)
            raw += query_vec[j] * rec.embedding[j];
        float effective = raw * rec.state.conductance;

        if (count < top_k) {
            best[count].id = rec.id; best[count].score = effective;
            count++;
            for (int j = count-1; j>0 && best[j].score > best[j-1].score; --j)
                std::swap(best[j], best[j-1]);
        } else if (effective > best[count-1].score) {
            best[count-1].id = rec.id; best[count-1].score = effective;
            for (int j = count-1; j>0 && best[j].score > best[j-1].score; --j)
                std::swap(best[j], best[j-1]);
        }
    }

    // 事件驱动演化
    if (g_evolution_enabled) {
        int actual_top = std::min(count, top_k);
        uint64_t selected_ids[10];
        for (int i = 0; i < actual_top; ++i) selected_ids[i] = best[i].id;

        for (auto& rec : store->records) {
            if (rec.embedding.empty() || (int)rec.embedding.size() != dim) continue;
            bool is_retrieved = false;
            for (int i = 0; i < actual_top; ++i) {
                if (rec.id == selected_ids[i]) { is_retrieved = true; break; }
            }
            if (is_retrieved) {
                rec.state.conductance += 0.05f * (1.0f - rec.state.conductance);
                rec.state.correlation_flux += 1.0f;
            } else {
                rec.state.conductance -= 0.01f;
            }
            if (rec.state.conductance > 1.0f) rec.state.conductance = 1.0f;
            if (rec.state.conductance < 0.0f) rec.state.conductance = 0.0f;
        }
    }

    for (int i = 0; i < count; ++i) {
        out_ids[i] = best[i].id;
        out_scores[i] = best[i].score;
    }
    return count;
}

extern "C" int prism_search(
    PrismStore* store, const float* query_vec, int dim, int top_k,
    PrismResult* results, float min_score)
{
    uint64_t ids[10]; float scores[10];
    int n = prism_search_raw(store, query_vec, dim, top_k, ids, scores);
    int out = 0;
    for (int i = 0; i < n; ++i) {
        if (scores[i] < min_score) continue;
        for (auto& rec : store->records) {
            if (rec.id == ids[i]) {
                results[out].id = ids[i];
                results[out].score = scores[i];
                results[out].role = strdup(rec.role.c_str());
                results[out].content = strdup(rec.content.c_str());
                out++;
                break;
            }
        }
    }
    return out;
}

extern "C" void prism_free_results(PrismResult* results, int count) {
    for (int i = 0; i < count; ++i) {
        free(results[i].role);
        free(results[i].content);
    }
}

extern "C" int prism_get_all_states(PrismStore* store, MemristorInfo* infos, int max_count) {
    int count = 0;
    for (auto& rec : store->records) {
        if (count >= max_count) break;
        infos[count].id = rec.id;
        infos[count].conductance = rec.state.conductance;
        infos[count].correlation_flux = rec.state.correlation_flux;
        infos[count].last_update_ts = rec.state.last_update_ts;
        count++;
    }
    return count;
}

extern "C" int prism_count(PrismStore* store) { return store->records.size(); }

extern "C" void prism_set_evolution(PrismStore* store, int enable) {
    g_evolution_enabled = (enable != 0);
}

extern "C" int prism_text_to_vector(const char* text, float* out_vec, int max_dim) {
    auto vec = text_to_vector(text);
    int n = std::min((int)vec.size(), max_dim);
    memcpy(out_vec, vec.data(), n * sizeof(float));
    return n;
}

extern "C" int prism_vocab_size() {
    return (int)g_vocab.size();
}