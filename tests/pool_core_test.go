package agilepool_test

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
	"github.com/stretchr/testify/assert"
)

// TestPoolCloseIdempotent verifies that Close() is safe to call multiple times
// and that Submit becomes a no-op after the pool is closed. A closed pool must
// still allow Wait() to return promptly for in-flight tasks.
func TestPoolCloseIdempotent(t *testing.T) {
	p := agilepool.NewPool()
	p.InitConfig().WithWorkerNumCapacity(10)
	p.Init()

	var executed int64
	for i := 0; i < 100; i++ {
		p.Submit(agilepool.TaskFunc(func() error {
			atomic.AddInt64(&executed, 1)
			return nil
		}))
	}
	p.Wait()
	assert.Equal(t, int64(100), atomic.LoadInt64(&executed))

	// Close multiple times — must not panic.
	p.Close()
	p.Close()
	p.Close()

	// Submit after Close should be silently dropped.
	var afterClose int64
	p.Submit(agilepool.TaskFunc(func() error {
		atomic.AddInt64(&afterClose, 1)
		return nil
	}))

	// Wait should return immediately since wg.Add was skipped for the
	// post-close submission.
	p.Wait()
	assert.Equal(t, int64(0), atomic.LoadInt64(&afterClose))
}

// TestPoolNonBlockMode verifies that when the pool is at capacity in NONBLOCK
// mode, overflow tasks are silently dropped rather than queued.
func TestPoolNonBlockMode(t *testing.T) {
	p := agilepool.NewPool()
	p.InitConfig().
		WithWorkerNumCapacity(1).
		WithBlockMode(agilepool.NONBLOCK)
	p.Init()

	var executed int64
	barrier := make(chan struct{})

	// First task holds the only worker.
	p.Submit(agilepool.TaskFunc(func() error {
		atomic.AddInt64(&executed, 1)
		<-barrier // blocked until released
		return nil
	}))

	// Submit many tasks from concurrent goroutines; they should all be
	// dropped by the NONBLOCK path since the single worker is busy.
	var submitWG sync.WaitGroup
	for i := 0; i < 50000; i++ {
		submitWG.Add(1)
		go func() {
			defer submitWG.Done()
			p.Submit(agilepool.TaskFunc(func() error {
				atomic.AddInt64(&executed, 1)
				return nil
			}))
		}()
	}
	submitWG.Wait()

	// Release the blocking task so the pool can drain.
	close(barrier)
	p.Wait()

	// Only the first task should have executed.
	assert.Equal(t, int64(1), atomic.LoadInt64(&executed))
}

// TestPoolWorkerPanicRecovery verifies that a panic inside a worker task does
// not crash the pool and that wg.Done() is still called so Wait() can return.
func TestPoolWorkerPanicRecovery(t *testing.T) {
	p := agilepool.NewPool()
	p.InitConfig().WithWorkerNumCapacity(1)
	p.Init()

	var executed int64

	p.Submit(agilepool.TaskFunc(func() error {
		atomic.AddInt64(&executed, 1)
		panic("intentional panic in worker")
	}))

	done := make(chan struct{})
	go func() {
		p.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Wait() timed out: panic may have prevented wg.Done from being called")
	}

	assert.Equal(t, int64(1), atomic.LoadInt64(&executed))
}

// TestPoolIdleWorkerReuse verifies that when tasks complete and workers become
// idle, subsequent submits reuse those idle workers from the idle container
// instead of creating new ones.
func TestPoolIdleWorkerReuse(t *testing.T) {
	p := agilepool.NewPool()
	p.InitConfig().
		WithWorkerNumCapacity(1).
		WithCleanPeriod(10 * time.Second) // prevent idle cleaner from interfering
	p.Init()

	var executed int64

	for i := 0; i < 5; i++ {
		p.Submit(agilepool.TaskFunc(func() error {
			atomic.AddInt64(&executed, 1)
			return nil
		}))
		// Wait for the task to finish. After Wait returns the worker has
		// been parked in idleWorks, so the next Submit will Pop it instead
		// of allocating a new worker from sync.Pool.
		p.Wait()
	}

	assert.Equal(t, int64(5), atomic.LoadInt64(&executed))
	// Only the first Submit created a new worker via sync.Pool.New;
	// the remaining 4 Submits reused the idle worker.
	assert.Equal(t, int64(1), atomic.LoadInt64(&p.WorkerCreateCount))
}

// TestPoolBlockModeQueuesTasks verifies that in BLOCK mode, tasks overflow into
// the taskQueue channel when all workers are busy, and execute once a worker
// drains the queue.
func TestPoolBlockModeQueuesTasks(t *testing.T) {
	p := agilepool.NewPool()
	p.InitConfig().
		WithWorkerNumCapacity(1).
		WithBlockMode(agilepool.BLOCK).
		WithTaskQueueSize(60000)
	p.Init()

	var executed int64
	barrier := make(chan struct{})

	// First task blocks the only worker.
	p.Submit(agilepool.TaskFunc(func() error {
		<-barrier
		return nil
	}))

	// Submit tasks in BLOCK mode; they queue up because the worker is busy.
	for i := 0; i < 50000; i++ {
		p.Submit(agilepool.TaskFunc(func() error {
			atomic.AddInt64(&executed, 1)
			return nil
		}))
	}

	// Release the worker so it drains the queue.
	close(barrier)
	p.Wait()
	assert.Equal(t, int64(50000), atomic.LoadInt64(&executed))
}

// TestPoolExpiredWorkerCleaner verifies that the background cleaner goroutine
// removes workers that have been idle beyond the expiry period (1s), forcing
// new worker creation for subsequent tasks.
func TestPoolExpiredWorkerCleaner(t *testing.T) {
	p := agilepool.NewPool()
	p.InitConfig().
		WithWorkerNumCapacity(1).
		WithCleanPeriod(50 * time.Millisecond) // fast cleanup loop
	p.Init()

	var executed int64

	// Submit and wait → worker goes idle in idleWorks.
	p.Submit(agilepool.TaskFunc(func() error {
		atomic.AddInt64(&executed, 1)
		return nil
	}))
	p.Wait()
	assert.Equal(t, int64(1), atomic.LoadInt64(&executed))
	assert.Equal(t, int64(1), atomic.LoadInt64(&p.WorkerCreateCount))

	// Wait for the cleaner to expire the idle worker (>1s idle threshold).
	time.Sleep(1100 * time.Millisecond)

	// Submit again → idleWorks is empty, must create a new worker.
	p.Submit(agilepool.TaskFunc(func() error {
		atomic.AddInt64(&executed, 1)
		return nil
	}))
	p.Wait()

	assert.Equal(t, int64(2), atomic.LoadInt64(&executed))
	// A new worker was needed because the cleaner removed the first one.
	assert.Equal(t, int64(2), atomic.LoadInt64(&p.WorkerCreateCount))
}

// TestPoolGetRunningWorkersNum verifies that GetRunningWorkersNum accurately
// reflects the number of workers currently executing tasks.
func TestPoolGetRunningWorkersNum(t *testing.T) {
	p := agilepool.NewPool()
	p.InitConfig().
		WithWorkerNumCapacity(5).
		WithCleanPeriod(10 * time.Second) // prevent cleaner from interfering
	p.Init()

	assert.Equal(t, int64(0), p.GetRunningWorkersNum())

	barrier := make(chan struct{})

	// Submit 3 blocking tasks to keep workers alive.
	for i := 0; i < 3; i++ {
		p.Submit(agilepool.TaskFunc(func() error {
			<-barrier
			return nil
		}))
	}

	// Submit already returns after incrementing runningWorkersNum, so
	// the count is correct immediately without needing a sleep.
	assert.Equal(t, int64(3), p.GetRunningWorkersNum())

	// Release all workers and verify they drain.
	close(barrier)
	p.Wait()

	// Give worker goroutines time to finish their idle transition and
	// decrement runningWorkersNum after wg.Done().
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int64(0), p.GetRunningWorkersNum())
}

// TestPoolSetLogger verifies that a custom logger captures pool output,
// confirming that SetLogger correctly replaces the default logger.
func TestPoolSetLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	p := agilepool.NewPool()
	p.SetLogger(logger)
	p.InitConfig().WithWorkerNumCapacity(1)
	p.Init()

	// Trigger a panic to verify logger output goes to buf via runTask's recover.
	// See worker.runTask: "worker exits from panic: %v\n%s\n"
	p.Submit(agilepool.TaskFunc(func() error {
		panic("verify logger")
	}))

	done := make(chan struct{})
	go func() {
		p.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Wait() deadlocked after panic")
	}

	// Wait returns before recover writes the log entry (defer order: done > recover),
	// so poll until the logger output is available.
	deadline := time.Now().Add(500 * time.Millisecond)
	for !strings.Contains(buf.String(), "worker exits from panic") ||
		!strings.Contains(buf.String(), "verify logger") {
		if time.Now().After(deadline) {
			t.Fatalf("logger output not captured within deadline, buf=%q", buf.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestPoolMinHeapContainer verifies that the MinHeap idle container type
// correctly supports worker reuse, matching the behavior already tested
// for LinkedList in TestPoolIdleWorkerReuse.
func TestPoolMinHeapContainer(t *testing.T) {
	p := agilepool.NewPool()
	p.InitConfig().
		WithWorkerNumCapacity(1).
		WithIdleContainerType(agilepool.MinHeapType).
		WithCleanPeriod(10 * time.Second)
	p.Init()

	var executed int64

	for i := 0; i < 5; i++ {
		p.Submit(agilepool.TaskFunc(func() error {
			atomic.AddInt64(&executed, 1)
			return nil
		}))
		p.Wait()
	}

	assert.Equal(t, int64(5), atomic.LoadInt64(&executed))
	assert.Equal(t, int64(1), atomic.LoadInt64(&p.WorkerCreateCount))
}