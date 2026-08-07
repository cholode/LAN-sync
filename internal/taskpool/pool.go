package taskpool

import (
	"lan-im-go/pkg"
	"sync"

	"github.com/panjf2000/ants/v2"
)

// Pool 全局协程池，用于限制并发、复用 goroutine
type Pool struct {
	pool *ants.Pool
}

var (
	globalPool     *Pool
	globalPoolOnce sync.Once
)

// Init 初始化全局协程池，应在 main 中尽早调用
// workerSize: 工作协程数，0 表示使用默认值（CPU 核数的 2 倍）
func Init(workerSize int) {
	globalPoolOnce.Do(func() {
		if workerSize <= 0 {
			workerSize = 256
		}

		opts := []ants.Option{
			ants.WithPreAlloc(true),
			ants.WithPanicHandler(func(i interface{}) {
				pkg.Errorf("[TaskPool] panic recovered: %v", i)
			}),
		}

		pool, err := ants.NewPool(workerSize, opts...)
		if err != nil {
			pkg.Fatalf("[TaskPool] 初始化失败: %v", err)
		}
		globalPool = &Pool{pool: pool}
		pkg.Infof("[TaskPool] 协程池已初始化, capacity=%d, running=%d", pool.Cap(), pool.Running())
	})
}

// Submit 提交任务到全局协程池，非阻塞
// 池满时会阻塞等待，区别于 juggle-im 的丢弃策略
func Submit(task func()) error {
	if globalPool == nil {
		go task()
		return nil
	}
	return globalPool.pool.Submit(task)
}

// Go 提交任务到协程池，无返回值（便捷方法）
func Go(task func()) {
	if err := Submit(task); err != nil {
		pkg.Errorf("[TaskPool] 提交失败: %v", err)
	}
}

// Running 返回当前正在运行的任务数
func Running() int {
	if globalPool == nil {
		return 0
	}
	return globalPool.pool.Running()
}

// Waiting 返回等待队列中的任务数
func Waiting() int {
	if globalPool == nil {
		return 0
	}
	return globalPool.pool.Waiting()
}

// Cap 返回池容量
func Cap() int {
	if globalPool == nil {
		return 0
	}
	return globalPool.pool.Cap()
}

// Release 释放协程池，等待所有任务完成后关闭
func Release() {
	if globalPool != nil {
		globalPool.pool.Release()
		pkg.Infoln("[TaskPool] 协程池已释放")
	}
}