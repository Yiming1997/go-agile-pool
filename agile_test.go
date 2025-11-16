package agilepool_test

import (
	"sync/atomic"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
	"github.com/stretchr/testify/assert"
)

func TestAgilePoolWorkerCapacityLimit(t *testing.T) {
	agilePool := agilepool.NewPool()
	agilePool.InitConfig().WithWorkerNumCapacity(10000)
	agilePool.Init()

	var maxWorkerNum int = 0

	for i := 0; i < 20000000; i++ {

		go func() {
			agilePool.Submit(
				agilepool.TaskFunc(func() {
					if int(agilePool.GetRunningWorkersNum()) > maxWorkerNum {
						maxWorkerNum = int(agilePool.GetRunningWorkersNum())
					}
					time.Sleep(10 * time.Millisecond)
				}),
			)
		}()

	}
	agilePool.Wait()
	assert.LessOrEqual(t, maxWorkerNum, 10000)
}

func TestAgilePoolWorkerCompletion(t *testing.T) {

	var sum int64
	sum = 0
	agilePool := agilepool.NewPool()
	agilePool.InitConfig().WithWorkerNumCapacity(10000)
	agilePool.Init()

	for i := 0; i < 1000000; i++ {

		go func() {
			agilePool.Submit(
				agilepool.TaskFunc(func() {
					atomic.AddInt64(&sum, int64(1))
				}),
			)
		}()

	}

	agilePool.Wait()

	assert.Equal(t, sum, int64(1000000))
}
