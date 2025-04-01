package util

import (
	"sync"
)

// FLock 用于防止函数重复调用的互斥锁工具
type FLock struct {
	mu    sync.Mutex
	inUse bool
}

// NewFLock 创建一个新的 FunctionLock 实例
func NewFLock() *FLock {
	return &FLock{}
}

// Should 判断函数是否应该继续执行
// 返回 true 表示可以执行，false 表示函数正在执行中
func (fl *FLock) Should() bool {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.inUse {
		return false
	}

	fl.inUse = true
	return true
}

// Done 标记函数执行完成
func (fl *FLock) Done() {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	fl.inUse = false
}
