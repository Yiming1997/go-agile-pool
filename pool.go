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
	DefaultCleanPeriod          = 100 * time.Millisecond
	DefaultTaskQueueSize        = 10000
	DefaultMaxWorkerNumCapacity = math.MaxInt64
	DefaultWorkMode             = BLOCK
	DefaultIdleContainerType    = LinkedListType
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
	muIdle            sync.Mutex
	workerPool        sync.Pool // Worker object pool
	idleWorks         IdleWorkerContainer
	config            *Config
	lock              *sync.Mutex
	wg                sync.WaitGroup
	logger            Logger
}

func NewPool() *Pool {
	p := &Pool{
		closePoolCn: make(chan struct{}),
		config:      &Config{},
		lock:        &sync.Mutex{},
		logger:      log.Default(),
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

	if p.config.idleContainerType == MinHeapType {
		p.idleWorks = newMinHeap()
	} else {
		p.idleWorks = newLinkedList()
	}

	p.capacity = p.config.workerNumCapacity
	p.taskQueue = make(chan Task, p.config.taskQueueSize)

	go p.expiredWorkerCleaner()
}

func (p *Pool) Submit(task Task) {
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

func (p *Pool) SubmitBefore(task Task, time time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), time)
	p.Submit(
		TaskFunc(func() error {
			select {
			case <-ctx.Done():
				cancel()
			default:
				task.Process()
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

	for range ticker.C {
		p.muIdle.Lock()
		p.idleWorks.RemoveExpired(time.Now(), 1*time.Second)
		p.muIdle.Unlock()
		runtime.Gosched()
	}
}

func (p *Pool) Close() {
	close(p.closePoolCn)
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

func (p *Pool) Done() {
	p.wg.Done()
}

func (p *Pool) GetRunningWorkersNum() int64 {
	return atomic.LoadInt64(&p.runningWorkersNum)
}
