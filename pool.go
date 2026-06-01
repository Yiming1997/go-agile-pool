package agilepool

import (
	"context"
	"log"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultCleanPeriod          = 100 * time.Millisecond
	defaultTaskQueueSize        = 10000
	defaultMaxWorkerNumCapacity = math.MaxInt64
	defaultWorkMode             = BLOCK
	defaultIdleContainerType    = LinkedListType
)

type WorkMode int8

const (
	BLOCK WorkMode = iota
	NONBLOCK
)

// Logger defines the logging interface used by the pool.
// Both the standard library's *log.Logger and structured loggers
// (e.g. zap.SugaredLogger) satisfy this interface.
type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type Pool struct {
	taskQueue         chan Task
	closePoolCn       chan struct{}
	capacity          int64 // The maximum number of workers in the pool.
	runningWorkersNum int64
	closed            int32 // 1 once Close has been called, otherwise 0
	muIdle            sync.Mutex
	workerPool        sync.Pool // Worker object pool
	idleWorks         IdleWorkerContainer
	config            *Config
	lock              *sync.Mutex
	wg                sync.WaitGroup
	logger            Logger
	// WorkerCreateCount counts the total allocations from sync.Pool.New
	// over the pool lifetime. For the number of currently active workers,
	// use GetRunningWorkersNum().
	WorkerCreateCount int64
}

func NewPool(c *Config) *Pool {
	if c == nil {
		c = NewConfig()
	}
	p := &Pool{
		closePoolCn: make(chan struct{}),
		config:      c,
		lock:        &sync.Mutex{},
		logger:      log.Default(),
		capacity:    c.workerNumCapacity,
		taskQueue:   make(chan Task, c.taskQueueSize),
	}

	if c.idleContainerType == MinHeapType {
		p.idleWorks = newMinHeap()
	} else {
		p.idleWorks = newLinkedList()
	}

	atomic.StoreInt64(&p.WorkerCreateCount, 0)

	p.workerPool.New = func() interface{} {
		atomic.AddInt64(&p.WorkerCreateCount, 1)
		w := &worker{
			pool: p,
		}
		return w
	}

	go p.expiredWorkerCleaner()
	return p
}

// SetLogger replaces the default standard-library logger.
// Pass the same logger instance used elsewhere in your application
// (e.g. zap.SugaredLogger) so pool output appears in the same log stream.
func (p *Pool) SetLogger(l Logger) {
	p.logger = l
}

func (p *Pool) Submit(task Task) {
	// Reject new submissions once Close has been called. We check before
	// wg.Add so that a closed pool's Wait() can still return promptly for
	// in-flight tasks and is not blocked by post-close submissions.
	if atomic.LoadInt32(&p.closed) == 1 {
		return
	}
	p.wg.Add(1)
	p.lock.Lock()

	if atomic.LoadInt64(&p.runningWorkersNum) < p.capacity {
		p.addRunningWorkersNum(1)
		p.lock.Unlock()
		p.muIdle.Lock()
		w := p.idleWorks.Pop()
		p.muIdle.Unlock()
		if w != nil {
			go w.run(task)
		} else {
			w := p.workerPool.Get().(*worker)
			go w.run(task)
		}
		return
	}
	p.lock.Unlock()
	if p.config.workMode == NONBLOCK {
		p.wg.Done()
		return
	}

	p.taskQueue <- task

	// Safety net: if workers exited between our capacity check and the push
	// above, tasks could be stranded in the channel buffer with no consumer.
	// Re-check under lock and spawn enough workers to drain the queue, up to
	// capacity. See TestQueueStuckRace for the race this guards against.
	p.lock.Lock()
	running := atomic.LoadInt64(&p.runningWorkersNum)
	target := int64(len(p.taskQueue))
	if target > p.capacity {
		target = p.capacity
	}
	toSpawn := target - running
	if toSpawn > 0 {
		p.addRunningWorkersNum(toSpawn)
		p.lock.Unlock()
		for i := int64(0); i < toSpawn; i++ {
			w := p.workerPool.Get().(*worker)
			go w.run(nil)
		}
		return
	}
	p.lock.Unlock()
}

// Submits a task before the specified timeout. If timeout is reached during execution, the task is canceled.
func (p *Pool) SubmitBefore(task Task, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	p.Submit(
		TaskFunc(func() error {
			defer cancel() // Ensures context is released after task completes to avoid resource leak
			select {
			case <-ctx.Done():
				return nil // Timeout reached, exit early
			default:
				task.Process() // Execute the task
			}
			return nil
		}),
	)
}

func (p *Pool) addToIdle(w *worker) {
	p.muIdle.Lock()
	defer p.muIdle.Unlock()
	p.idleWorks.Add(w)
}

func (p *Pool) addRunningWorkersNum(num int64) {
	atomic.AddInt64(&p.runningWorkersNum, num)
}

func (p *Pool) expiredWorkerCleaner() {
	ticker := time.NewTicker(p.config.cleanPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.muIdle.Lock()
			p.idleWorks.RemoveExpired(time.Now(), 1*time.Second)
			p.muIdle.Unlock()
			runtime.Gosched()
		case <-p.closePoolCn:
			return
		}
	}
}

// Close marks the pool as closed and stops its background cleaner goroutine.
// After Close:
//   - new Submit calls become no-ops (the task is dropped, no goroutine is started)
//   - in-flight tasks already submitted continue to run to completion
//   - Wait() returns once all in-flight tasks are done, enabling graceful shutdown
//
// Close is idempotent and safe to call from any goroutine, including from
// within a running task.
func (p *Pool) Close() {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return
	}
	close(p.closePoolCn)
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

func (p *Pool) done() {
	p.wg.Done()
}

func (p *Pool) GetRunningWorkersNum() int64 {
	return atomic.LoadInt64(&p.runningWorkersNum)
}
