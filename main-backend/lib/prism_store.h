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
    float chaos;
    uint64_t last_update_ts;
} MemristorInfo;

PrismStore* prism_open(const char* data_path);
void prism_close(PrismStore* store);
uint64_t prism_insert(PrismStore* store, const char* role, const char* content,
                      const char* keywords, const float* embedding, int embedding_dim);
int prism_search(PrismStore* store, const float* query_vec, int dim, int top_k,
                 PrismResult* results, float min_score);
int prism_search_raw(PrismStore* store, const float* query_vec, int dim, int top_k,
                     uint64_t* out_ids, float* out_scores);
void prism_free_results(PrismResult* results, int count);
int prism_get_all_states(PrismStore* store, MemristorInfo* infos, int max_count);
int prism_count(PrismStore* store);
void prism_set_evolution(PrismStore* store, int enable);

#ifdef __cplusplus
}
#endif

#endif