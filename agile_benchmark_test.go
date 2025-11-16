package agilepool_test

import (
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
)

const (
	taskCount = 10000000
)

func BenchmarkAgilePool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool()
		pool.InitConfig().WithCleanPeriod(500 * time.Millisecond).WithTaskQueueSize(10000).WithWorkerNumCapacity(20000)
		pool.Init()

		for j := 0; j < taskCount; j++ {
			go func() {
				pool.Submit(agilepool.TaskFunc(func() {
					defer pool.Wg.Done()
					time.Sleep(10 * time.Millisecond)
				}))

			}()

		}
		pool.Wg.Wait()

		pool.Close()
	}
}
