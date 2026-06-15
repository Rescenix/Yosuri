#ifndef CELLULAR_ENGINE_H
#define CELLULAR_ENGINE_H

#ifdef __cplusplus
extern "C" {
#endif

// 不透明指针，隐藏 C++ 实现细节
typedef struct CellularEngine CellularEngine;

CellularEngine* cellular_create(int grid_size);
void cellular_destroy(CellularEngine* engine);
void cellular_set_cell(CellularEngine* engine, int x, int y, int strategy_id, float activation, float reward);
void cellular_step(CellularEngine* engine, float noise);
int cellular_winner(CellularEngine* engine);
void cellular_get_cell(CellularEngine* engine, int x, int y, int* strategy_id, float* activation);

#ifdef __cplusplus
}
#endif

#endif