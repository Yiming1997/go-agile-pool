package agilepool_test

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
	"github.com/stretchr/testify/assert"
)

func TestAgilePoolWorkerCapacityLimit(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10000),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))

	var maxWorkerNum int64

	n := 10000000
	if testing.Short() {
		n = 100000
	}

	var submitWG sync.WaitGroup
	for i := 0; i < n; i++ {
		submitWG.Add(1)
		go func() {
			defer submitWG.Done()
			agilePool.Submit(
				agilepool.TaskFunc(func() error {
					cur := int64(agilePool.GetRunningWorkersNum())
					for {
						old := atomic.LoadInt64(&maxWorkerNum)
						if cur <= old {
							break
						}
						if atomic.CompareAndSwapInt64(&maxWorkerNum, old, cur) {
							break
						}
					}
					time.Sleep(10 * time.Millisecond)
					return nil
				}),
			)
		}()
	}

	submitWG.Wait()
	agilePool.Wait()
	assert.LessOrEqual(t, atomic.LoadInt64(&maxWorkerNum), int64(10000))
}

func TestAgilePoolWorkerCompletion(t *testing.T) {
	var sum int64
	sum = 0
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10000),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))

	n := 1000000
	if testing.Short() {
		n = 10000
	}

	var submitWG sync.WaitGroup
	for i := 0; i < n; i++ {
		submitWG.Add(1)
		go func() {
			defer submitWG.Done()
			agilePool.Submit(
				agilepool.TaskFunc(func() error {
					atomic.AddInt64(&sum, int64(1))
					return nil
				}),
			)
		}()
	}

	submitWG.Wait()
	agilePool.Wait()

	assert.Equal(t, sum, int64(n))
}

func TestAgilePoolSubmitBeforeCompletion(t *testing.T) {
	var sum int64
	sum = 0
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10000),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))

	n := 1000000
	if testing.Short() {
		n = 10000
	}

	var submitWG sync.WaitGroup
	for i := 0; i < n; i++ {
		submitWG.Add(1)
		go func() {
			defer submitWG.Done()
			agilePool.SubmitBefore(
				agilepool.TaskFunc(func() error {
					time.Sleep(10 * time.Millisecond)
					atomic.AddInt64(&sum, int64(1))
					return nil
				}), 10*time.Second,
			)
		}()
	}

	submitWG.Wait()
	agilePool.Wait()
	assert.Equal(t, sum, int64(n))
}

func TestAgilePoolTaskRetryTimes(t *testing.T) {
	var times int64 = 0
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))

	minBackOff := 1 * time.Second
	maxBackOff := 200 * time.Second
	if testing.Short() {
		minBackOff = 1 * time.Millisecond
		maxBackOff = 10 * time.Millisecond
	}

	agilePool.Submit(&agilepool.TaskWithRetry{
		MinBackOff: minBackOff,
		MaxBackOff: maxBackOff,
		RetryNum:   3,
		Task: func() error {
			times++
			log.Println("getting err over here")
			return errors.New("err")
		},
	})

	agilePool.Wait()
	assert.Equal(t, times, int64(4))
}

// TestAgilePoolRaceStuckTaskInQueue reproduces a race condition where a task
// can be left stranded in the channel buffer with no consumer goroutine.
//
// Race window:
//  1. Submit locks, sees running==capacity, unlocks (line 127)
//  2. Worker finishes its task, select-loop sees empty queue, goes to default,
//     adds self to idleWorkers, goroutine exits, running-- (worker.go:63-65)
//  3. Submit pushes task into taskQueue (pool.go:133)
//     -> no goroutine is reading the channel, task is stuck forever
//
// The key insight: this bug is "self-healing" -- any subsequent Submit will
// spawn a new worker that drains the stuck task. Only the *last* Submit of
// a pool's lifetime can permanently lose a task. So we create many small
// isolated pool lifetimes to amplify the exposure.
func TestAgilePoolRaceStuckTaskInQueue(t *testing.T) {
	iterations := 5000
	batchSize := 200
	capacity := int64(1)
	deadline := 2 * time.Second

	if testing.Short() {
		iterations = 200
		deadline = 5 * time.Second
	}

	for iter := 0; iter < iterations; iter++ {
		p := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithWorkerNumCapacity(capacity),
			agilepool.WithTaskQueueSize(10000),
		))

		var executed int64
		var submitWG sync.WaitGroup

		// Submit batchSize tasks concurrently from batchSize goroutines.
		// High concurrency maximizes lock contention and widens the race window.
		for i := 0; i < batchSize; i++ {
			submitWG.Add(1)
			go func() {
				defer submitWG.Done()
				p.Submit(agilepool.TaskFunc(func() error {
					atomic.AddInt64(&executed, 1)
					return nil
				}))
			}()
		}

		// Wait until all Submit calls have returned.
		// After this point no more tasks will be submitted -- this is the
		// critical "end-of-life" moment where the race becomes visible.
		submitWG.Wait()

		// Run pool.Wait() in a goroutine; if it doesn't return within the
		// deadline, a task is stranded in the queue and wg.Done() was never
		// called for it -> the race triggered.
		done := make(chan struct{})
		go func() {
			p.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All tasks completed, this iteration is clean.
		case <-time.After(deadline):
			t.Fatalf("iter %d: DEADLOCK after %v, executed=%d/%d, runningWorkers=%d",
				iter, deadline, atomic.LoadInt64(&executed), batchSize,
				p.GetRunningWorkersNum())
		}
	}
}
