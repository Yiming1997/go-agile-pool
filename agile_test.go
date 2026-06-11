package agilepool_test

import (
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
	"github.com/stretchr/testify/assert"
)

func TestAgilePoolWorkerCapacityLimit(t *testing.T) {
	taskCount := 10000000
	workerCapacity := int64(10000)
	if testing.Short() {
		taskCount = 20000
		workerCapacity = 100
	}

	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(workerCapacity),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))
	defer agilePool.Close()

	var maxWorkerNum int64
	var submitWG sync.WaitGroup

	for i := 0; i < taskCount; i++ {
		submitWG.Add(1)
		go func() {
			defer submitWG.Done()
			agilePool.Submit(
				agilepool.TaskFunc(func() error {
					running := agilePool.GetRunningWorkersNum()
					for {
						currentMax := atomic.LoadInt64(&maxWorkerNum)
						if running <= currentMax ||
							atomic.CompareAndSwapInt64(&maxWorkerNum, currentMax, running) {
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
	assert.LessOrEqual(t, maxWorkerNum, workerCapacity)
}

func TestAgilePoolWorkerCompletion(t *testing.T) {
	taskCount := 1000000
	if testing.Short() {
		taskCount = 20000
	}

	var sum int64
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10000),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))
	defer agilePool.Close()

	var submitWG sync.WaitGroup
	for i := 0; i < taskCount; i++ {
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

	assert.Equal(t, int64(taskCount), sum)
}

func TestAgilePoolSubmitBeforeCompletion(t *testing.T) {
	taskCount := 1000000
	if testing.Short() {
		taskCount = 20000
	}

	var sum int64
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10000),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))
	defer agilePool.Close()

	var submitWG sync.WaitGroup
	for i := 0; i < taskCount; i++ {
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
	assert.Equal(t, int64(taskCount), sum)
}

func TestAgilePoolTaskRetryTimes(t *testing.T) {
	var times int64 = 0
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))

	agilePool.Submit(&agilepool.TaskWithRetry{
		MinBackOff: 1 * time.Second,
		MaxBackOff: 200 * time.Second,
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

func TestAgilePoolTaskPanicDoesNotBreakPool(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(1),
		agilepool.WithTaskQueueSize(10),
	))
	agilePool.SetLogger(log.New(io.Discard, "", 0))
	defer agilePool.Close()

	var executed int64

	agilePool.Submit(agilepool.TaskFunc(func() error {
		panic("boom")
	}))
	agilePool.Submit(agilepool.TaskFunc(func() error {
		atomic.AddInt64(&executed, 1)
		return nil
	}))

	done := make(chan struct{})
	go func() {
		agilePool.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pool.Wait() timed out after a task panic")
	}

	assert.Equal(t, int64(1), atomic.LoadInt64(&executed))

	agilePool.Submit(agilepool.TaskFunc(func() error {
		atomic.AddInt64(&executed, 1)
		return nil
	}))
	agilePool.Wait()

	assert.Equal(t, int64(2), atomic.LoadInt64(&executed))
}

// TestAgilePoolRaceStuckTaskInQueue reproduces a race condition where a task
// can be left stranded in the channel buffer with no consumer goroutine.
//
// Race window:
//  1. Submit locks, sees running==capacity, unlocks (line 127)
//  2. Worker finishes its task, select-loop sees empty queue, goes to default,
//     adds self to idleWorkers, goroutine exits, running-- (worker.go:63-65)
//  3. Submit pushes task into taskQueue (pool.go:133)
//     → no goroutine is reading the channel, task is stuck forever
//
// The key insight: this bug is "self-healing" — any subsequent Submit will
// spawn a new worker that drains the stuck task. Only the *last* Submit of
// a pool's lifetime can permanently lose a task. So we create many small
// isolated pool lifetimes to amplify the exposure.
func TestAgilePoolRaceStuckTaskInQueue(t *testing.T) {
	const (
		batchSize = 200
		capacity  = int64(1)
		deadline  = 2 * time.Second
	)

	iterations := 5000
	if testing.Short() {
		iterations = 250
	}

	tests := []struct {
		name          string
		containerType agilepool.IdleContainerType
	}{
		{name: "linked_list", containerType: agilepool.LinkedListType},
		{name: "min_heap", containerType: agilepool.MinHeapType},
		{name: "slice", containerType: agilepool.SliceType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for iter := 0; iter < iterations; iter++ {
				p := agilepool.NewPool(agilepool.NewConfig(
					agilepool.WithWorkerNumCapacity(capacity),
					agilepool.WithTaskQueueSize(10000),
					agilepool.WithIdleContainerType(tt.containerType),
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
				// After this point no more tasks will be submitted — this is the
				// critical "end-of-life" moment where the race becomes visible.
				submitWG.Wait()

				// Run pool.Wait() in a goroutine; if it doesn't return within the
				// deadline, a task is stranded in the queue and wg.Done() was never
				// called for it, so the race triggered.
				done := make(chan struct{})
				go func() {
					p.Wait()
					close(done)
				}()

				select {
				case <-done:
					p.Close()
				case <-time.After(deadline):
					p.Close()
					t.Fatalf("iter %d: DEADLOCK after %v, executed=%d/%d, runningWorkers=%d",
						iter, deadline, atomic.LoadInt64(&executed), batchSize,
						p.GetRunningWorkersNum())
				}
			}
		})
	}
}
