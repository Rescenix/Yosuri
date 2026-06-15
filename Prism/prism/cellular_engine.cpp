#include "cellular_engine.h"
#include <algorithm>
#include <cstdlib>
#include <ctime>
#include <unordered_map> 

CellularEngine::CellularEngine(int size) : grid_size(size) {
    cells.resize(size * size, {-1, 0.0f});
    rewards.resize(size * size, 0.5f);
    std::srand(std::time(nullptr));
}

void CellularEngine::setCell(int x, int y, int strategy_id, float initial_activation, float reward) {
    if (x < 0 || x >= grid_size || y < 0 || y >= grid_size) return;
    int idx = y * grid_size + x;
    cells[idx].strategy_id = strategy_id;
    cells[idx].activation = initial_activation;
    rewards[idx] = reward;
}

void CellularEngine::step(float noise) {
    std::vector<float> new_activations(cells.size());
    // 简单的局部博弈规则：每个元胞对比周围邻居的回报，调整自己的支持度
    for (int y = 0; y < grid_size; ++y) {
        for (int x = 0; x < grid_size; ++x) {
            int idx = y * grid_size + x;
            if (cells[idx].strategy_id < 0) continue; // 空元胞不参与更新
            
            float delta = 0.0f;
            int neighbor_count = 0;
            // 摩尔邻域
            for (int dy = -1; dy <= 1; ++dy) {
                for (int dx = -1; dx <= 1; ++dx) {
                    if (dx == 0 && dy == 0) continue;
                    int nx = (x + dx + grid_size) % grid_size;
                    int ny = (y + dy + grid_size) % grid_size;
                    int nidx = ny * grid_size + nx;
                    if (cells[nidx].strategy_id >= 0) {
                        delta += rewards[idx] - rewards[nidx];
                        ++neighbor_count;
                    }
                }
            }
            if (neighbor_count > 0) {
                delta /= neighbor_count;
                // 更新：sigmoid 式增强/减弱
                float change = 0.1f * delta;
                float new_act = cells[idx].activation + change;
                // 加入噪声
                new_act += noise * (static_cast<float>(std::rand()) / RAND_MAX - 0.5f);
           new_activations[idx] = (new_act < 0.0f) ? 0.0f : (new_act > 1.0f) ? 1.0f : new_act;
            } else {
                new_activations[idx] = cells[idx].activation;
            }
        }
    }
    
    for (size_t i = 0; i < cells.size(); ++i) {
        if (cells[i].strategy_id >= 0) {
            cells[i].activation = new_activations[i];
        }
    }
}

int CellularEngine::getWinner() const {
    // 统计每个策略的平均激活度
    std::unordered_map<int, float> sum_act;
    std::unordered_map<int, int> count;
    for (const auto& cell : cells) {
        if (cell.strategy_id >= 0) {
            sum_act[cell.strategy_id] += cell.activation;
            count[cell.strategy_id]++;
        }
    }
    int best_id = -1;
    float best_avg = -1.0f;
    for (const auto& p : sum_act) {
        float avg = p.second / count[p.first];
        if (avg > best_avg) {
            best_avg = avg;
            best_id = p.first;
        }
    }
    return best_id;
}

void CellularEngine::getCellInfo(int x, int y, int& strategy_id, float& activation) const {
    if (x < 0 || x >= grid_size || y < 0 || y >= grid_size) {
        strategy_id = -1;
        activation = 0.0f;
        return;
    }
    int idx = y * grid_size + x;
    strategy_id = cells[idx].strategy_id;
    activation = cells[idx].activation;
}

// C 接口实现
void* cellular_create(int grid_size) {
    return new CellularEngine(grid_size);
}
void cellular_destroy(void* engine) {
    delete static_cast<CellularEngine*>(engine);
}
void cellular_set_cell(void* engine, int x, int y, int strategy_id, float activation, float reward) {
    static_cast<CellularEngine*>(engine)->setCell(x, y, strategy_id, activation, reward);
}
void cellular_step(void* engine, float noise) {
    static_cast<CellularEngine*>(engine)->step(noise);
}
int cellular_winner(void* engine) {
    return static_cast<CellularEngine*>(engine)->getWinner();
}
void cellular_get_cell(void* engine, int x, int y, int* strategy_id, float* activation) {
    static_cast<CellularEngine*>(engine)->getCellInfo(x, y, *strategy_id, *activation);
}