package agilepool

import (
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultCleanPeriod          = 100 * time.Millisecond
	DefaultTaskQueueSize        = 10000
	DefaultMaxWorkerNumCapacity = math.MaxInt64
	DefaultWorkMode             = BLOCK
	DefaultIdleContainerType    = LinkedListType
	DefaultExpiryDuration       = 1 * time.Second
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
	capacity          int64 // The maximum number of workers in the pool.
	runningWorkersNum int64
	workerPool        sync.Pool // Worker object pool
	config            *Config
	wg                sync.WaitGroup
	logger            Logger
	closed            atomic.Bool
}

func NewPool() *Pool {
	p := &Pool{
		config: &Config{},
		logger: log.Default(),
	}

	p.workerPool.New = func() interface{} {
		w := &worker{
			pool: p,
		}
		return w
	}

	return p
}

func (p *Pool) InitConfig() (config *Config) {
	return p.config
}

// SetLogger replaces the default standard-library logger.
// Pass the same logger instance used elsewhere in your application
// (e.g. zap.SugaredLogger) so pool output appears in the same log stream.
func (p *Pool) SetLogger(l Logger) {
	p.logger = l
}

func (p *Pool) Init() {

	if p.config.cleanPeriod == 0 {
		p.config.cleanPeriod = DefaultCleanPeriod
	}

	if p.config.taskQueueSize == 0 {
		p.config.taskQueueSize = DefaultTaskQueueSize
	}

	if p.config.workerNumCapacity == 0 {
		p.config.workerNumCapacity = DefaultMaxWorkerNumCapacity
	}

	if p.config.workMode == 0 {
		p.config.workMode = DefaultWorkMode
	}

	if p.config.expiryDuration == 0 {
		p.config.expiryDuration = DefaultExpiryDuration
	}

	p.capacity = p.config.workerNumCapacity
	p.taskQueue = make(chan Task, p.config.taskQueueSize)
}

// Submit submits a task to the pool. In BLOCK mode it blocks until the task
// is enqueued; in NONBLOCK mode the task is silently dropped if the pool is full.
// Uses CAS (lock-free) for the worker-capacity check to avoid mutex contention
// under high concurrency.
func (p *Pool) Submit(task Task) {
	p.wg.Add(1)

	if p.closed.Load() {
		p.wg.Done()
		return
	}

	// Lock-free capacity check via CAS loop.
	for {
		current := atomic.LoadInt64(&p.runningWorkersNum)
		if current >= p.capacity {
			break // At capacity, enqueue via channel.
		}
		if atomic.CompareAndSwapInt64(&p.runningWorkersNum, current, current+1) {
			w := p.workerPool.Get().(*worker)
			go w.run(task)
			return
		}
		// CAS failed — another goroutine took the slot; retry.
	}

	if p.config.workMode == NONBLOCK {
		p.wg.Done()
		return
	}

	p.taskQueue <- task
}

// SubmitBefore submits a task with a deadline. If the task cannot be enqueued
// (because all workers are busy and the task queue is full) within the given
// duration, the task is dropped and WaitGroup is decremented.
func (p *Pool) SubmitBefore(task Task, timeout time.Duration) {
	p.wg.Add(1)

	if p.closed.Load() {
		p.wg.Done()
		return
	}

	for {
		current := atomic.LoadInt64(&p.runningWorkersNum)
		if current >= p.capacity {
			break
		}
		if atomic.CompareAndSwapInt64(&p.runningWorkersNum, current, current+1) {
			w := p.workerPool.Get().(*worker)
			go w.run(task)
			return
		}
	}

	if p.config.workMode == NONBLOCK {
		p.wg.Done()
		return
	}

	select {
	case p.taskQueue <- task:
	case <-time.After(timeout):
		p.wg.Done()
	}
}

func (p *Pool) addRunningWorkersNum(num int64) {
	atomic.AddInt64(&p.runningWorkersNum, num)
}

// Close gracefully shuts down the pool: waits for all submitted tasks to
// complete, then closes the task queue so workers exit their loops.
func (p *Pool) Close() {
	p.closed.Store(true)
	p.wg.Wait()
	close(p.taskQueue)
}

// Wait blocks until all submitted tasks have completed.
func (p *Pool) Wait() {
	p.wg.Wait()
}

func (p *Pool) GetRunningWorkersNum() int64 {
	return atomic.LoadInt64(&p.runningWorkersNum)
}