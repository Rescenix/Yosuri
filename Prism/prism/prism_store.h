#ifndef PRISM_STORE_H
#define PRISM_STORE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct PrismStore PrismStore;

typedef struct {
    uint64_t id;
    float score;
    char* role;
    char* content;
} PrismResult;

typedef struct {
    uint64_t id;
    float conductance;
    float correlation_flux;
    uint64_t last_update_ts;
} MemristorInfo;

// 创建/销毁存储
PrismStore* prism_open(const char* data_path);
void prism_close(PrismStore* store);

// 插入一条记录（传入文本内容和手工特征向量；若 embedding 为 NULL 则使用内部 TF‑IDF）
uint64_t prism_insert(PrismStore* store, const char* role, const char* content,
                      const char* keywords, const float* embedding, int embedding_dim);

// 搜索（top_k 控制返回数量，min_score 为最低相似度）
int prism_search(PrismStore* store, const float* query_vec, int dim, int top_k,
                 PrismResult* results, float min_score);

// 释放搜索结果中的动态字符串
void prism_free_results(PrismResult* results, int count);

// 获取记录总数
int prism_count(PrismStore* store);

// 获取所有记录的电导/通量信息
int prism_get_all_states(PrismStore* store, MemristorInfo* infos, int max_count);

// 控制事件驱动演化（1=开启，0=关闭）
void prism_set_evolution(PrismStore* store, int enable);

// 文本向量化接口（基于内部词库生成 TF‑IDF 向量）
int prism_text_to_vector(const char* text, float* out_vec, int max_dim);

// 返回当前词库维度
int prism_vocab_size();

#ifdef __cplusplus
}
#endif

#endif