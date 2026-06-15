#ifndef CELLULAR_ENGINE_H
#define CELLULAR_ENGINE_H

#include <cstdint>
#include <vector>

struct Cell {
    int strategy_id;    // 策略标识，-1 表示空
    float activation;   // 当前支持度 [0, 1]
};

class CellularEngine {
public:
    CellularEngine(int grid_size);
    
    // 在指定位置设置策略和回报（回报将用于迭代更新）
    void setCell(int x, int y, int strategy_id, float initial_activation, float reward);
    
    // 执行一次迭代，noise 为随机扰动强度 [0, 0.1]
    void step(float noise = 0.0f);
    
    // 获取当前获胜的策略 ID（平均激活度最高）
    int getWinner() const;
    
    // 获取指定位置的信息
    void getCellInfo(int x, int y, int& strategy_id, float& activation) const;

private:
    int grid_size;
    std::vector<Cell> cells;
    std::vector<float> rewards;   // 每个元胞绑定的策略回报
};

// C 接口
extern "C" {
    void* cellular_create(int grid_size);
    void cellular_destroy(void* engine);
    void cellular_set_cell(void* engine, int x, int y, int strategy_id, float activation, float reward);
    void cellular_step(void* engine, float noise);
    int cellular_winner(void* engine);
    void cellular_get_cell(void* engine, int x, int y, int* strategy_id, float* activation);
}
#endif