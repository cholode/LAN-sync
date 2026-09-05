package taskpool

import (
	"sync"

	"github.com/panjf2000/ants/v2"

	"lan-im-go/pkg"
)

// Pool 是可独立持有的协程池，用于限制并发并复用 goroutine。
type Pool struct {
	pool *ants.Pool
}

// New 创建独立协程池。服务或组件应优先持有自己的协程池，
// 避免与进程级兼容协程池争抢容量。
func New(workerSize int) (*Pool, error) {
	if workerSize <= 0 {
		workerSize = 512
	}
	pool, err := ants.NewPool(
		workerSize,
		ants.WithPreAlloc(true),
		ants.WithPanicHandler(func(value interface{}) {
			pkg.Errorf("[TaskPool] panic recovered: %v", value)
		}),
	)
	if err != nil {
		return nil, err
	}
	return &Pool{pool: pool}, nil
}

func (p *Pool) Submit(task func()) error {
	if p == nil || p.pool == nil {
		go task()
		return nil
	}
	return p.pool.Submit(task)
}

func (p *Pool) Running() int {
	if p == nil || p.pool == nil {
		return 0
	}
	return p.pool.Running()
}

func (p *Pool) Waiting() int {
	if p == nil || p.pool == nil {
		return 0
	}
	return p.pool.Waiting()
}

func (p *Pool) Cap() int {
	if p == nil || p.pool == nil {
		return 0
	}
	return p.pool.Cap()
}

func (p *Pool) Release() {
	if p != nil && p.pool != nil {
		p.pool.Release()
	}
}

var (
	globalPool     *Pool
	globalPoolOnce sync.Once
)

// Init 初始化全局协程池，应在 main 中尽早调用
// workerSize: 工作协程数，0 表示使用默认值
func Init(workerSize int) {
	globalPoolOnce.Do(func() {
		pool, err := New(workerSize)
		if err != nil {
			pkg.Fatalf("[TaskPool] 初始化失败: %v", err)
		}
		globalPool = pool
		pkg.Infof("[TaskPool] 协程池已初始化, capacity=%d, running=%d", pool.Cap(), pool.Running())
	})
}

// Submit 向进程级兼容协程池提交任务。
// 池满时会阻塞等待，不会直接丢弃任务。
func Submit(task func()) error {
	if globalPool == nil {
		go task()
		return nil
	}
	return globalPool.Submit(task)
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
	return globalPool.Running()
}

// Waiting 返回等待队列中的任务数
func Waiting() int {
	if globalPool == nil {
		return 0
	}
	return globalPool.Waiting()
}

// Cap 返回池容量
func Cap() int {
	if globalPool == nil {
		return 0
	}
	return globalPool.Cap()
}

// Release 释放协程池，等待所有任务完成后关闭
func Release() {
	if globalPool != nil {
		globalPool.Release()
		pkg.Infoln("[TaskPool] 协程池已释放")
	}
}
