package taskpool

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	Init(4)
}

func TestSubmit_Basic(t *testing.T) {
	var counter int32
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		Go(func() {
			defer wg.Done()
			atomic.AddInt32(&counter, 1)
		})
	}

	wg.Wait()
	if counter != 10 {
		t.Errorf("counter = %d, want 10", counter)
	}
}

func TestSubmit_PanicRecovery(t *testing.T) {
	var recovered sync.WaitGroup
	recovered.Add(1)

	Go(func() {
		defer recovered.Done()
		panic("test panic recovery")
	})

	recovered.Wait()
}

func TestCap(t *testing.T) {
	if Cap() != 4 {
		t.Errorf("Cap() = %d, want 4", Cap())
	}
}

func TestRunning(t *testing.T) {
	var started sync.WaitGroup
	var done sync.WaitGroup
	blocker := make(chan struct{})

	for i := 0; i < 2; i++ {
		started.Add(1)
		done.Add(1)
		Go(func() {
			started.Done()
			<-blocker
			done.Done()
		})
	}

	// 等待所有任务被调度
	started.Wait()
	time.Sleep(20 * time.Millisecond)

	r := Running()
	if r < 2 {
		t.Errorf("Running() = %d, want at least 2", r)
	}

	close(blocker)
	done.Wait()
}
