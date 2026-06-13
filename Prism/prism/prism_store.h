#ifndef PRISM_STORE_H
#define PRISM_STORE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct PrismStore PrismStore;

// 打开存储，若文件不存在则创建
PrismStore* prism_open(const char* data_path);

// 关闭并释放
void prism_close(PrismStore* store);

// 插入一条记忆，返回自增 ID
uint64_t prism_insert(
    PrismStore* store,
    const char* role,
    const char* content,
    const char* keywords,
    const float* embedding,
    int embedding_dim
);

// 暴力搜索，返回 top_k 个结果（按余弦相似度降序）
typedef struct {
    uint64_t id;
    float score;          // 点积（假设向量已归一化）
    char* role;
    char* content;
} PrismResult;

int prism_search(
    PrismStore* store,
    const float* query_vec,
    int dim,
    int top_k,
    PrismResult* results,  // 由调用方分配 top_k 个结构体
    float min_score
);

// 释放搜索结果中分配的字符串
void prism_free_results(PrismResult* results, int count);

// 获取总记忆数
int prism_count(PrismStore* store);

#ifdef __cplusplus
}
#endif

#endif