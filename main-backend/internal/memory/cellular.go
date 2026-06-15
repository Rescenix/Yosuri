package memory

/*
#cgo LDFLAGS: -L${SRCDIR}/../../lib -lprism_store -static -lstdc++ -lgcc -lwinpthread
#include "cellular_engine.h"
*/
import "C"
import (
	"math/rand"
)

type CellEngine struct {
	handle *C.CellularEngine
	size   int
}

func NewCellEngine(gridSize int) *CellEngine {
	return &CellEngine{
		handle: C.cellular_create(C.int(gridSize)),
		size:   gridSize,
	}
}

func (e *CellEngine) Destroy() {
	C.cellular_destroy(e.handle)
}

func (e *CellEngine) SetCell(x, y, strategyID int, activation, reward float32) {
	C.cellular_set_cell(e.handle, C.int(x), C.int(y), C.int(strategyID), C.float(activation), C.float(reward))
}

func (e *CellEngine) Step(noise float32) {
	C.cellular_step(e.handle, C.float(noise))
}

func (e *CellEngine) Winner() int {
	return int(C.cellular_winner(e.handle))
}

type Candidate struct {
	ID     int
	Reward float32
}

// RunStrategyGame 运行一次元胞博弈，返回获胜策略ID
func RunStrategyGame(candidates []Candidate, gridSize int, iterations int, noise float32) int {
	engine := NewCellEngine(gridSize)
	defer engine.Destroy()

	for _, c := range candidates {
		x := rand.Intn(gridSize)
		y := rand.Intn(gridSize)
		engine.SetCell(x, y, c.ID, 0.5, c.Reward)
	}
	for i := 0; i < iterations; i++ {
		engine.Step(noise)
	}
	return engine.Winner()
}
