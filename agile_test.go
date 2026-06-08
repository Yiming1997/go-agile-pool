package agilepool_test

import (
	"context"
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

func TestAgilePoolSubmitNilTaskDoesNotBlockWait(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig())
	defer agilePool.Close()

	agilePool.Submit(nil)
	agilePool.Wait()
}

func TestAgilePoolSubmitTypedNilTaskDoesNotBlockWait(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig())
	defer agilePool.Close()

	var task agilepool.TaskFunc
	agilePool.Submit(task)
	agilePool.Wait()
}

func TestAgilePoolSubmitBeforeNilTaskDoesNotBlockWait(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig())
	defer agilePool.Close()

	agilePool.SubmitBefore(nil, time.Second)
	agilePool.Wait()
}

func TestAgilePoolSubmitCtxCanceledBeforeSubmitDoesNotExecute(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig())
	defer agilePool.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var executed int64
	agilePool.SubmitCtx(ctx, agilepool.TaskFunc(func() error {
		atomic.AddInt64(&executed, 1)
		return nil
	}))
	agilePool.Wait()

	assert.Equal(t, int64(0), atomic.LoadInt64(&executed))
}

func TestAgilePoolSubmitCtxCanceledWhileQueuedSkipsTask(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(1),
		agilepool.WithTaskQueueSize(10),
	))
	defer agilePool.Close()

	started := make(chan struct{})
	release := make(chan struct{})
	agilePool.Submit(agilepool.TaskFunc(func() error {
		close(started)
		<-release
		return nil
	}))
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	var executed int64
	agilePool.SubmitCtx(ctx, agilepool.TaskFunc(func() error {
		atomic.AddInt64(&executed, 1)
		return nil
	}))
	cancel()
	close(release)
	agilePool.Wait()

	assert.Equal(t, int64(0), atomic.LoadInt64(&executed))
}

func TestAgilePoolSubmitCtxRunningTaskObservesCancel(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig())
	defer agilePool.Close()

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	var canceled int64

	agilePool.SubmitCtx(ctx, agilepool.TaskFunc(func() error {
		close(started)
		<-ctx.Done()
		atomic.AddInt64(&canceled, 1)
		return ctx.Err()
	}))
	<-started
	cancel()
	agilePool.Wait()

	assert.Equal(t, int64(1), atomic.LoadInt64(&canceled))
}

func TestAgilePoolSubmitCtxCancelsWhileWaitingForQueueSpace(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(1),
		agilepool.WithTaskQueueSize(1),
	))
	defer agilePool.Close()

	started := make(chan struct{})
	release := make(chan struct{})
	agilePool.Submit(agilepool.TaskFunc(func() error {
		close(started)
		<-release
		return nil
	}))
	<-started

	agilePool.Submit(agilepool.TaskFunc(func() error {
		return nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	var executed int64
	go func() {
		agilePool.SubmitCtx(ctx, agilepool.TaskFunc(func() error {
			atomic.AddInt64(&executed, 1)
			return nil
		}))
		close(returned)
	}()

	cancel()
	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("SubmitCtx did not return after context cancellation")
	}

	close(release)
	agilePool.Wait()

	assert.Equal(t, int64(0), atomic.LoadInt64(&executed))
}

func TestAgilePoolWorkerCapacityLimit(t *testing.T) {
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10000),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))

	var maxWorkerNum int = 0

	for i := 0; i < 10000000; i++ {

		go func() {
			agilePool.Submit(
				agilepool.TaskFunc(func() error {
					if int(agilePool.GetRunningWorkersNum()) > maxWorkerNum {
						maxWorkerNum = int(agilePool.GetRunningWorkersNum())
					}
					time.Sleep(10 * time.Millisecond)
					return nil
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
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10000),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))

	for i := 0; i < 1000000; i++ {

		go func() {
			agilePool.Submit(
				agilepool.TaskFunc(func() error {
					atomic.AddInt64(&sum, int64(1))
					return nil
				}),
			)
		}()

	}

	agilePool.Wait()

	assert.Equal(t, sum, int64(1000000))
}

func TestAgilePoolSubmitBeforeCompletion(t *testing.T) {
	var sum int64
	sum = 0
	agilePool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithWorkerNumCapacity(10000),
		agilepool.WithIdleContainerType(agilepool.MinHeapType),
	))

	for i := 0; i < 1000000; i++ {

		go func() {
			agilePool.SubmitBefore(
				agilepool.TaskFunc(func() error {
					time.Sleep(10 * time.Millisecond)
					atomic.AddInt64(&sum, int64(1))
					return nil
				}), 10*time.Second,
			)
		}()

	}

	agilePool.Wait()
	assert.Equal(t, sum, int64(1000000))
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
		iterations = 5000
		batchSize  = 200
		capacity   = int64(1)
		deadline   = 2 * time.Second
	)

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
		// After this point no more tasks will be submitted — this is the
		// critical "end-of-life" moment where the race becomes visible.
		submitWG.Wait()

		// Run pool.Wait() in a goroutine; if it doesn't return within the
		// deadline, a task is stranded in the queue and wg.Done() was never
		// called for it → the race triggered.
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
