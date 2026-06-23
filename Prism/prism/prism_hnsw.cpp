// prism_hnsw.cpp — Prism-HNSW 实现（完全解耦，无 PrismStore 依赖）
#include "prism_hnsw.h"
#include <cmath>
#include <queue>
#include <unordered_set>
#include <unordered_map>
#include <random>
#include <algorithm>
#include <chrono>
#include <cstdio>   // 新增：用于文件读写

struct HNSWNode {
    uint64_t id;
    int level;
    std::vector<std::vector<uint64_t>> neighbors;
};

struct PrismHNSWHandle {
    int M, efConstruction, efSearch, maxLevel;
    double mL;
    uint64_t entryPoint;
    std::vector<HNSWNode> nodes;
    std::unordered_map<uint64_t, int> idToIndex;
    std::unordered_map<uint64_t, std::vector<float>> vectors;
    std::mt19937 rng;

    PrismHNSWHandle(int m, int efC, int efS)
        : M(m), efConstruction(efC), efSearch(efS), maxLevel(-1),
          mL(1.0 / std::log(1.0 * m)), entryPoint(0) {
        rng.seed(std::chrono::steady_clock::now().time_since_epoch().count());
    }

    int randomLevel() {
        std::uniform_real_distribution<double> dist(0.0, 1.0);
        return (int)(-std::log(dist(rng)) * mL);
    }

    float cosineSimilarity(const float* a, const float* b, int dim) {
        float dot = 0, na = 0, nb = 0;
        for (int i = 0; i < dim; ++i) {
            dot += a[i] * b[i];
            na += a[i] * a[i];
            nb += b[i] * b[i];
        }
        if (na == 0 || nb == 0) return 0;
        return dot / (std::sqrt(na) * std::sqrt(nb));
    }

    std::vector<std::pair<uint64_t, float>> searchLayer(
        const float* query, uint64_t entry, int ef, int layer) {
        std::unordered_set<uint64_t> visited;
        auto cmp = [](const std::pair<uint64_t, float>& a, const std::pair<uint64_t, float>& b) {
            return a.second < b.second;
        };
        std::priority_queue<std::pair<uint64_t, float>,
            std::vector<std::pair<uint64_t, float>>, decltype(cmp)> candidates(cmp);
        std::priority_queue<std::pair<uint64_t, float>,
            std::vector<std::pair<uint64_t, float>>, decltype(cmp)> results(cmp);

        float dist = 1.0f - cosineSimilarity(query, vectors[entry].data(), (int)vectors[entry].size());
        candidates.push({entry, dist});
        results.push({entry, dist});
        visited.insert(entry);

        while (!candidates.empty()) {
            auto curr = candidates.top(); candidates.pop();
            if (curr.second > results.top().second) break;

            int idx = idToIndex[curr.first];
            for (uint64_t neighbor : nodes[idx].neighbors[layer]) {
                if (visited.count(neighbor)) continue;
                visited.insert(neighbor);
                float ndist = 1.0f - cosineSimilarity(query, vectors[neighbor].data(), (int)vectors[neighbor].size());
                if (ndist < results.top().second || results.size() < ef) {
                    candidates.push({neighbor, ndist});
                    results.push({neighbor, ndist});
                    if (results.size() > ef) results.pop();
                }
            }
        }

        std::vector<std::pair<uint64_t, float>> resultList;
        while (!results.empty()) { resultList.push_back(results.top()); results.pop(); }
        std::reverse(resultList.begin(), resultList.end());
        return resultList;
    }
};

extern "C" {

PrismHNSWHandle* prism_hnsw_create(int m, int ef_construction, int ef_search) {
    return new PrismHNSWHandle(m, ef_construction, ef_search);
}

void prism_hnsw_destroy(PrismHNSWHandle* handle) {
    delete handle;
}

void prism_hnsw_insert(PrismHNSWHandle* handle, uint64_t id, const float* vec, int dim) {
    if (!handle) return;
    handle->vectors[id] = std::vector<float>(vec, vec + dim);

    int level = handle->randomLevel();
    HNSWNode node;
    node.id = id;
    node.level = level;
    node.neighbors.resize(level + 1);

    int idx = handle->nodes.size();
    handle->nodes.push_back(node);
    handle->idToIndex[id] = idx;

    if (handle->entryPoint == 0 && handle->nodes.size() == 1) {
        handle->entryPoint = id;
        handle->maxLevel = level;
        return;
    }

    uint64_t currEntry = handle->entryPoint;
    for (int lc = handle->maxLevel; lc > level; lc--) {
        currEntry = handle->searchLayer(vec, currEntry, 1, lc)[0].first;
    }

    for (int lc = std::min(level, handle->maxLevel); lc >= 0; lc--) {
        auto candidates = handle->searchLayer(vec, currEntry, handle->efConstruction, lc);
        if (candidates.size() > handle->M) candidates.resize(handle->M);

        int currIdx = handle->idToIndex[id];
        for (auto& cand : candidates) {
            int candIdx = handle->idToIndex[cand.first];
            handle->nodes[currIdx].neighbors[lc].push_back(cand.first);
            handle->nodes[candIdx].neighbors[lc].push_back(id);
        }
        currEntry = candidates[0].first;
    }

    if (level > handle->maxLevel) {
        handle->maxLevel = level;
        handle->entryPoint = id;
    }
}

int prism_hnsw_search(PrismHNSWHandle* handle, const float* query, int dim, int k,
                      uint64_t* out_ids, float* out_distances) {
    if (!handle || handle->nodes.empty()) return 0;

    uint64_t currEntry = handle->entryPoint;
    for (int lc = handle->maxLevel; lc > 0; lc--) {
        currEntry = handle->searchLayer(query, currEntry, 1, lc)[0].first;
    }
    auto results = handle->searchLayer(query, currEntry, handle->efSearch, 0);
    if (results.size() > k) results.resize(k);

    for (size_t i = 0; i < results.size(); ++i) {
        out_ids[i] = results[i].first;
        out_distances[i] = results[i].second;
    }
    return (int)results.size();
}

void prism_hnsw_clear(PrismHNSWHandle* handle) {
    if (!handle) return;
    handle->nodes.clear();
    handle->idToIndex.clear();
    handle->vectors.clear();
    handle->entryPoint = 0;
    handle->maxLevel = -1;
}

// ========== 持久化实现 ==========

void prism_hnsw_save(PrismHNSWHandle* handle, const char* path) {
    if (!handle) return;
    FILE* f = fopen(path, "wb");
    if (!f) return;

    // 头部信息
    fwrite(&handle->entryPoint, sizeof(uint64_t), 1, f);
    fwrite(&handle->maxLevel, sizeof(int), 1, f);
    int nodeCount = (int)handle->nodes.size();
    fwrite(&nodeCount, sizeof(int), 1, f);

    // 每个节点的信息
    for (auto& node : handle->nodes) {
        fwrite(&node.id, sizeof(uint64_t), 1, f);
        fwrite(&node.level, sizeof(int), 1, f);
        for (int l = 0; l <= node.level; l++) {
            int nCount = (int)node.neighbors[l].size();
            fwrite(&nCount, sizeof(int), 1, f);
            fwrite(node.neighbors[l].data(), sizeof(uint64_t), nCount, f);
        }
    }

    // 向量数据
    int vecCount = (int)handle->vectors.size();
    fwrite(&vecCount, sizeof(int), 1, f);
    for (auto& kv : handle->vectors) {
        fwrite(&kv.first, sizeof(uint64_t), 1, f);
        int dim = (int)kv.second.size();
        fwrite(&dim, sizeof(int), 1, f);
        fwrite(kv.second.data(), sizeof(float), dim, f);
    }

    fclose(f);
    printf("HNSW 索引已保存到 %s\n", path);
}

bool prism_hnsw_load(PrismHNSWHandle* handle, const char* path) {
    if (!handle) return false;
    FILE* f = fopen(path, "rb");
    if (!f) return false;

    // 清空现有数据
    handle->nodes.clear();
    handle->idToIndex.clear();
    handle->vectors.clear();

    // 读取头部
    fread(&handle->entryPoint, sizeof(uint64_t), 1, f);
    fread(&handle->maxLevel, sizeof(int), 1, f);
    int nodeCount;
    fread(&nodeCount, sizeof(int), 1, f);

    // 读取节点
    handle->nodes.resize(nodeCount);
    for (int i = 0; i < nodeCount; i++) {
        auto& node = handle->nodes[i];
        fread(&node.id, sizeof(uint64_t), 1, f);
        fread(&node.level, sizeof(int), 1, f);
        node.neighbors.resize(node.level + 1);
        for (int l = 0; l <= node.level; l++) {
            int nCount;
            fread(&nCount, sizeof(int), 1, f);
            node.neighbors[l].resize(nCount);
            fread(node.neighbors[l].data(), sizeof(uint64_t), nCount, f);
        }
        handle->idToIndex[node.id] = i;
    }

    // 读取向量
    int vecCount;
    fread(&vecCount, sizeof(int), 1, f);
    for (int i = 0; i < vecCount; i++) {
        uint64_t id;
        int dim;
        fread(&id, sizeof(uint64_t), 1, f);
        fread(&dim, sizeof(int), 1, f);
        std::vector<float> vec(dim);
        fread(vec.data(), sizeof(float), dim, f);
        handle->vectors[id] = vec;
    }

    fclose(f);
    printf("HNSW 索引已从 %s 加载 (%d 节点)\n", path, nodeCount);
    return true;
}

} // extern "C"