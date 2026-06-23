// prism_store.cpp
#include "prism_store.h"
#include <vector>
#include <cstring>
#include <cstdio>
#include <algorithm>
#include <chrono>
#include <string>
#include <cmath>
#include <cstdlib>

const float SIGMA = 10.0f;
const float RHO = 28.0f;
const float BETA = 8.0f / 3.0f;
const float DT = 0.005f;
const int LORENZ_STEPS = 30;
static bool g_evolution_enabled = true;

struct ChaoticState {
    float X = 0.5f, Y = 0.5f, Z = 0.5f, conductance = 0.5f, correlation_flux = 0.0f;
    uint64_t last_update_ts = 0;
};

struct MemoryRecord {
    uint64_t id;
    std::string role, content, keywords;
    std::vector<float> embedding;
    ChaoticState chaos;
};

struct PrismStore {
    std::string data_path;
    FILE* file;
    std::vector<MemoryRecord> records;
    uint64_t next_id = 1;
    ~PrismStore() { if (file) fclose(file); }
};

static uint64_t now_ms() {
    return std::chrono::duration_cast<std::chrono::milliseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();
}

inline void update_lorenz_state(MemoryRecord& rec, float disturbance, uint64_t current_time) {
    if (disturbance > 0.0001f) rec.chaos.X += disturbance * 0.1f;
    for (int i = 0; i < LORENZ_STEPS; ++i) {
        float dx = SIGMA * (rec.chaos.Y - rec.chaos.X);
        float dy = rec.chaos.X * (RHO - rec.chaos.Z) - rec.chaos.Y;
        float dz = rec.chaos.X * rec.chaos.Y - BETA * rec.chaos.Z;
        rec.chaos.X += dx * DT;
        rec.chaos.Y += dy * DT;
        rec.chaos.Z += dz * DT;
    }
    rec.chaos.conductance = 1.0f / (1.0f + exp(-rec.chaos.X * 0.5f));
    if (disturbance > 0.0001f) rec.chaos.correlation_flux += 1.0f;
    rec.chaos.last_update_ts = current_time;
}

static void append_to_file(FILE* f, const MemoryRecord& rec) {
    fwrite(&rec.id, sizeof(uint64_t), 1, f);
    uint32_t len;
    len = rec.role.size(); fwrite(&len, sizeof(uint32_t), 1, f); fwrite(rec.role.data(), 1, len, f);
    len = rec.content.size(); fwrite(&len, sizeof(uint32_t), 1, f); fwrite(rec.content.data(), 1, len, f);
    len = rec.keywords.size(); fwrite(&len, sizeof(uint32_t), 1, f); fwrite(rec.keywords.data(), 1, len, f);
    uint32_t dim = rec.embedding.size(); fwrite(&dim, sizeof(uint32_t), 1, f);
    fwrite(rec.embedding.data(), sizeof(float), dim, f);
    fwrite(&rec.chaos.X, sizeof(float), 1, f);
    fwrite(&rec.chaos.Y, sizeof(float), 1, f);
    fwrite(&rec.chaos.Z, sizeof(float), 1, f);
    fwrite(&rec.chaos.conductance, sizeof(float), 1, f);
    fwrite(&rec.chaos.correlation_flux, sizeof(float), 1, f);
    fwrite(&rec.chaos.last_update_ts, sizeof(uint64_t), 1, f);
    fflush(f);
}

static size_t read_record(FILE* f, MemoryRecord& rec) {
    if (fread(&rec.id, sizeof(uint64_t), 1, f) != 1) return 0;
    auto read_str = [&](std::string& s) {
        uint32_t len; if (fread(&len, sizeof(uint32_t), 1, f) != 1) return false;
        s.resize(len); return fread(&s[0], 1, len, f) == len;
    };
    if (!read_str(rec.role) || !read_str(rec.content) || !read_str(rec.keywords)) return 0;
    uint32_t dim; if (fread(&dim, sizeof(uint32_t), 1, f) != 1) return 0;
    rec.embedding.resize(dim);
    if (fread(rec.embedding.data(), sizeof(float), dim, f) != dim) return 0;
    fread(&rec.chaos.X, sizeof(float), 1, f);
    fread(&rec.chaos.Y, sizeof(float), 1, f);
    fread(&rec.chaos.Z, sizeof(float), 1, f);
    fread(&rec.chaos.conductance, sizeof(float), 1, f);
    fread(&rec.chaos.correlation_flux, sizeof(float), 1, f);
    fread(&rec.chaos.last_update_ts, sizeof(uint64_t), 1, f);
    return 1;
}

extern "C" int prism_get_content(PrismStore* store, uint64_t id, char* out_role, int role_len, char* out_content, int content_len) {
    memset(out_role, 0, role_len);
    memset(out_content, 0, content_len);
    for (auto& rec : store->records) {
        if (rec.id == id) {
            strncpy(out_role, rec.role.c_str(), role_len - 1);
            strncpy(out_content, rec.content.c_str(), content_len - 1);
            return 0;
        }
    }
    return -1;
}

extern "C" PrismStore* prism_open(const char* data_path) {
    auto* store = new PrismStore();
    store->data_path = data_path;
    store->file = fopen(data_path, "a+b");
    if (!store->file) { delete store; return nullptr; }
    fseek(store->file, 0, SEEK_SET);
    MemoryRecord rec;
    while (read_record(store->file, rec)) {
        store->records.push_back(std::move(rec));
        if (rec.id >= store->next_id) store->next_id = rec.id + 1;
    }
    fseek(store->file, 0, SEEK_END);
    return store;
}

extern "C" void prism_close(PrismStore* store) { delete store; }

extern "C" uint64_t prism_insert(
    PrismStore* store,
    const char* role, const char* content,
    const char* keywords,
    const float* embedding, int embedding_dim)
{
    MemoryRecord rec;
    rec.id = store->next_id++;
    rec.role = role;
    rec.content = content;
    rec.keywords = keywords ? keywords : "";

    if (embedding && embedding_dim > 0)
        rec.embedding.assign(embedding, embedding + embedding_dim);
    else
        return 0;

    float seed = (store->next_id % 100) * 0.0001f;
    rec.chaos.X = 0.5f + seed;
    rec.chaos.Y = 0.5f - seed * 0.7f;
    rec.chaos.Z = 0.5f + seed * 0.3f;
    rec.chaos.conductance = 0.5f;
    rec.chaos.correlation_flux = 0.0f;
    rec.chaos.last_update_ts = now_ms();

    append_to_file(store->file, rec);
    store->records.push_back(std::move(rec));
    return rec.id;
}

extern "C" int prism_search_raw(
    PrismStore* store, const float* query_vec, int dim, int top_k,
    uint64_t* out_ids, float* out_scores)
{
    struct RawScore { uint64_t id; float score; };
    RawScore best[10]; int count = 0;
    for (auto& rec : store->records) {
        if (rec.embedding.empty() || (int)rec.embedding.size() != dim) continue;
        float raw = 0.0f;
        for (int j = 0; j < dim; ++j) raw += query_vec[j] * rec.embedding[j];
        float effective = raw * rec.chaos.conductance;
        if (count < top_k) {
            best[count] = {rec.id, effective}; count++;
            for (int j = count-1; j>0 && best[j].score > best[j-1].score; --j) std::swap(best[j], best[j-1]);
        } else if (effective > best[count-1].score) {
            best[count-1] = {rec.id, effective};
            for (int j = count-1; j>0 && best[j].score > best[j-1].score; --j) std::swap(best[j], best[j-1]);
        }
    }

    if (g_evolution_enabled) {
        for (int i = 0; i < count; ++i) {
            for (auto& rec : store->records) {
                if (rec.id == best[i].id) {
                    update_lorenz_state(rec, best[i].score * 0.05f, now_ms());
                    break;
                }
            }
        }
    }

    for (int i = 0; i < count; ++i) {
        out_ids[i] = best[i].id;
        out_scores[i] = best[i].score;
    }
    return count;
}

extern "C" int prism_search(PrismStore* store, const float* query_vec, int dim, int top_k,
                            PrismResult* results, float min_score) {
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
        infos[count].conductance = rec.chaos.conductance;
        infos[count].correlation_flux = rec.chaos.correlation_flux;
        float mag = sqrt(rec.chaos.Y*rec.chaos.Y + rec.chaos.Z*rec.chaos.Z);
        infos[count].chaos = 1.0f / (1.0f + exp(-mag * 0.3f));
        infos[count].last_update_ts = rec.chaos.last_update_ts;
        count++;
    }
    return count;
}

extern "C" int prism_count(PrismStore* store) { return store->records.size(); }
extern "C" void prism_set_evolution(PrismStore* store, int enable) { g_evolution_enabled = (enable != 0); }