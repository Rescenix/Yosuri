#ifndef CHAOS_CORE_H
#define CHAOS_CORE_H

#include <cstdint>

struct ChaoticState {
    float X = 0.5f;
    float Y = 0.5f;
    float Z = 0.5f;
    float conductance = 0.5f;
    float correlation_flux = 0.0f;
    uint64_t last_update_ts = 0;
};

// 单步洛伦兹演化（可加微小扰动）
void lorenz_step(ChaoticState& state, float disturbance = 0.0f);

// 检索扰动：增加通量，更新状态
void apply_retrieval_disturbance(ChaoticState& state, float score, uint64_t now);

#endif