package agilepool_test

import (
	"sync"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
)

const (
	workerCount = 10000
	taskCount   = 10000000
)

func BenchmarkEfficientPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool()
		pool.InitConfig().WithCleanPeriod(500 * time.Millisecond).WithTaskQueueSize(10000).WithWorkerNumCapacity(20000)
		pool.Init()

		var wg sync.WaitGroup
		wg.Add(taskCount)

		for j := 0; j < taskCount; j++ {
			go func() {
				pool.Submit(agilepool.TaskFunc(func() {
					time.Sleep(10 * time.Millisecond)

					wg.Done()
				}))
			}()

		}

		wg.Wait()
		pool.Close()
	}
}
