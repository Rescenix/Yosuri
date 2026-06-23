// prism_hnsw.h — Prism-HNSW 独立索引库
#pragma once
#include <cstdint>
#include <vector>
#include <utility>

#ifdef __cplusplus
extern "C" {
#endif

// 不透明句柄
typedef struct PrismHNSWHandle PrismHNSWHandle;

// 创建 HNSW 索引
PrismHNSWHandle* prism_hnsw_create(int m, int ef_construction, int ef_search);

// 销毁索引
void prism_hnsw_destroy(PrismHNSWHandle* handle);

// 插入向量
void prism_hnsw_insert(PrismHNSWHandle* handle, uint64_t id, const float* vec, int dim);

// 搜索 topK 个最近邻
int prism_hnsw_search(PrismHNSWHandle* handle, const float* query, int dim, int k,
                      uint64_t* out_ids, float* out_distances);

// 重建索引（清空并重新开始）
void prism_hnsw_clear(PrismHNSWHandle* handle);

// 保存索引到文件
void prism_hnsw_save(PrismHNSWHandle* handle, const char* path);

// 从文件加载索引
bool prism_hnsw_load(PrismHNSWHandle* handle, const char* path);

#ifdef __cplusplus
}
#endif