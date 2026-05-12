package agilepool_test

import (
	"errors"
	"log"
	"sync/atomic"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
	"github.com/stretchr/testify/assert"
)

func TestAgilePoolWorkerCapacityLimit(t *testing.T) {
	agilePool := agilepool.NewPool()
	agilePool.InitConfig().WithWorkerNumCapacity(10000).WithExpiryDuration(30 * time.Second)
	agilePool.Init()

	var maxWorkerNum atomic.Int64

	for i := 0; i < 1000000; i++ {
		go func() {
			agilePool.Submit(
				agilepool.TaskFunc(func() error {
					current := agilePool.GetRunningWorkersNum()
					for {
						old := maxWorkerNum.Load()
						if current <= old || maxWorkerNum.CompareAndSwap(old, current) {
							break
						}
					}
					time.Sleep(10 * time.Millisecond)
					return nil
				}),
			)
		}()
	}
	agilePool.Wait()
	assert.LessOrEqual(t, int(maxWorkerNum.Load()), 10000)
}

func TestAgilePoolWorkerCompletion(t *testing.T) {
	var sum atomic.Int64
	agilePool := agilepool.NewPool()
	agilePool.InitConfig().WithWorkerNumCapacity(10000).WithExpiryDuration(30 * time.Second)
	agilePool.Init()

	for i := 0; i < 1000000; i++ {
		go func() {
			agilePool.Submit(
				agilepool.TaskFunc(func() error {
					sum.Add(1)
					return nil
				}),
			)
		}()
	}

	agilePool.Wait()
	assert.Equal(t, sum.Load(), int64(1000000))
}

func TestAgilePoolSubmitBeforeCompletion(t *testing.T) {
	var sum atomic.Int64
	agilePool := agilepool.NewPool()
	agilePool.InitConfig().WithWorkerNumCapacity(10000).WithExpiryDuration(30 * time.Second)
	agilePool.Init()

	for i := 0; i < 1000000; i++ {
		go func() {
			agilePool.SubmitBefore(
				agilepool.TaskFunc(func() error {
					time.Sleep(10 * time.Millisecond)
					sum.Add(1)
					return nil
				}), 10*time.Second,
			)
		}()
	}

	agilePool.Wait()
	assert.Equal(t, sum.Load(), int64(1000000))
}

func TestAgilePoolTaskRetryTimes(t *testing.T) {
	var times atomic.Int64

	agilePool := agilepool.NewPool()
	agilePool.InitConfig().WithWorkerNumCapacity(10).WithExpiryDuration(30 * time.Second)
	agilePool.Init()

	agilePool.Submit(&agilepool.TaskWithRetry{
		MinBackOff: 1 * time.Millisecond,
		MaxBackOff: 200 * time.Millisecond,
		RetryNum:   3,
		Task: func() error {
			times.Add(1)
			log.Println("getting err over here")
			return errors.New("err")
		},
		Pool: agilePool,
	})

	// Wait for retries to complete (they are re-submitted with backoff delay).
	// Total attempts = 1 (initial) + 3 (retries) = 4.
	time.Sleep(5 * time.Second)
	assert.Equal(t, times.Load(), int64(4))
	agilePool.Wait()
}

func TestClose(t *testing.T) {
	agilePool := agilepool.NewPool()
	agilePool.InitConfig().WithWorkerNumCapacity(1000).WithExpiryDuration(30 * time.Second)
	agilePool.Init()

	var sum atomic.Int64
	for i := 0; i < 100; i++ {
		agilePool.Submit(
			agilepool.TaskFunc(func() error {
				sum.Add(1)
				return nil
			}),
		)
	}

	agilePool.Close()
	assert.Equal(t, sum.Load(), int64(100))
}

func TestConcurrentSubmit(t *testing.T) {
	agilePool := agilepool.NewPool()
	agilePool.InitConfig().WithWorkerNumCapacity(500).WithExpiryDuration(30 * time.Second)
	agilePool.Init()

	var maxRunning atomic.Int64
	var completed atomic.Int64
	const totalTasks = 100000

	for i := 0; i < totalTasks; i++ {
		go func() {
			agilePool.Submit(
				agilepool.TaskFunc(func() error {
					current := agilePool.GetRunningWorkersNum()
					for {
						old := maxRunning.Load()
						if current <= old || maxRunning.CompareAndSwap(old, current) {
							break
						}
					}
					completed.Add(1)
					return nil
				}),
			)
		}()
	}

	agilePool.Wait()
	assert.LessOrEqual(t, int(maxRunning.Load()), 500)
	assert.Equal(t, completed.Load(), int64(totalTasks))
}

func TestSubmitBeforeTimeout(t *testing.T) {
	agilePool := agilepool.NewPool()
	agilePool.InitConfig().
		WithWorkerNumCapacity(5).
		WithTaskQueueSize(1).
		WithExpiryDuration(30 * time.Second)
	agilePool.Init()

	var executed atomic.Int64

	// Submit tasks that block workers for a long time, filling all 5 workers.
	for i := 0; i < 5; i++ {
		agilePool.Submit(
			agilepool.TaskFunc(func() error {
				time.Sleep(3 * time.Second)
				executed.Add(1)
				return nil
			}),
		)
	}

	// Now submit tasks with a very short timeout.
	// With only 1 queue slot and all 5 workers busy, most will be dropped.
	// We count how many actually execute (should be ≤ 6: 5 blocking + at most 1 from queue).
	for i := 0; i < 50; i++ {
		agilePool.SubmitBefore(
			agilepool.TaskFunc(func() error {
				executed.Add(1)
				return nil
			}),
			10*time.Millisecond,
		)
	}

	agilePool.Wait()
	// The 5 blocking tasks should always complete.
	// The short-deadline tasks may or may not get in, depending on timing.
	assert.GreaterOrEqual(t, executed.Load(), int64(5))
	assert.LessOrEqual(t, executed.Load(), int64(6))
}