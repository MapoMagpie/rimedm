package util

import "sync"

type IDGenerator struct {
	currentID uint8
	mutex     sync.Mutex
}

var (
	IDGen *IDGenerator = &IDGenerator{currentID: 0}
)

func (gen *IDGenerator) NextID() uint8 {
	gen.mutex.Lock()
	defer gen.mutex.Unlock()

	gen.currentID++
	return gen.currentID
}

func (gen *IDGenerator) Reset() {
	gen.mutex.Lock()
	defer gen.mutex.Unlock()

	gen.currentID = 0
}
