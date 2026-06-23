#include "chaos_core.h"
#include <cmath>

namespace {
    constexpr float SIGMA = 10.0f;
    constexpr float RHO   = 28.0f;
    constexpr float BETA  = 8.0f / 3.0f;
    constexpr float DT    = 0.005f;
    constexpr int   STEPS = 30;
}

static void run_lorenz(ChaoticState& state, int steps) {
    for (int i = 0; i < steps; ++i) {
        float dx = SIGMA * (state.Y - state.X);
        float dy = state.X * (RHO - state.Z) - state.Y;
        float dz = state.X * state.Y - BETA * state.Z;
        state.X += dx * DT;
        state.Y += dy * DT;
        state.Z += dz * DT;
    }
    state.conductance = 1.0f / (1.0f + std::exp(-state.X * 0.5f));
}

void lorenz_step(ChaoticState& state, float disturbance) {
    state.X += disturbance * 0.1f;
    run_lorenz(state, STEPS);
}

void apply_retrieval_disturbance(ChaoticState& state, float score, uint64_t now) {
    state.X += score * 0.1f;
    run_lorenz(state, STEPS);
    state.correlation_flux += 1.0f;
    state.last_update_ts = now;
}