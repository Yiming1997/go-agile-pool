package agilepool_test

import (
	agilepool "agilePool"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAgilePoolWorkerCapacityLimit(t *testing.T) {
	agilePool := agilepool.NewPool()
	agilePool.InitConfig().WithWorkerNumCapacity(10000)
	agilePool.Init()

	var wg sync.WaitGroup

	var maxWorkerNum int = 0

	for i := 0; i < 2000000; i++ {
		wg.Add(1)

		go func() {
			agilePool.Submit(func() {
				defer wg.Done()
				if int(agilePool.GetRunningWorkersNum()) > maxWorkerNum {
					maxWorkerNum = int(agilePool.GetRunningWorkersNum())
				}
				time.Sleep(10 * time.Millisecond)
			})
		}()

	}
	wg.Wait()
	assert.LessOrEqual(t, maxWorkerNum, 20000)

}
