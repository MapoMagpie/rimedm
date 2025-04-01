package util

import (
	"sync"
	"time"
)

// Debouncer 防抖结构体
type Debouncer struct {
	duration time.Duration
	timer    *time.Timer
	mu       sync.Mutex
}

// New 创建一个新的防抖实例
func NewDebouncer(duration time.Duration) *Debouncer {
	return &Debouncer{
		duration: duration,
	}
}

// Debounce 防抖方法
func (d *Debouncer) Do(f func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 如果已有定时器，先停止
	if d.timer != nil {
		d.timer.Stop()
	}

	// 设置新的定时器
	d.timer = time.AfterFunc(d.duration, f)
}

// 使用示例：
/*
func main() {
	debouncer := debounce.New(500 * time.Millisecond)

	for i := 0; i < 10; i++ {
		debouncer.Debounce(func() {
			fmt.Println("Executed after last call!")
		})
		time.Sleep(100 * time.Millisecond)
	}

	// 等待足够长时间以确保最后一次执行
	time.Sleep(600 * time.Millisecond)
}
*/
