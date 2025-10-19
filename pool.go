package agilepool

import (
	"context"
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
)

type Pool struct {
	taskQueue         chan Task
	closePoolCn       chan struct{}
	capacity          int64 // The maximum number of workers in the pool.
	runningWorkersNum int64
	muIdle            sync.Mutex
	workerPool        sync.Pool // Worker object pool
	idleWorks         *LinkedList[*worker]
	config            *Config
	lock              sync.Locker
}

func NewPool() *Pool {
	p := &Pool{
		closePoolCn: make(chan struct{}),
		idleWorks:   newLinkedList[*worker](),
		config:      &Config{},
		lock:        &sync.Mutex{},
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

	p.capacity = p.config.workerNumCapacity
	p.taskQueue = make(chan Task, p.config.taskQueueSize)

	go p.expiredWorkerCleaner()
}

func (p *Pool) Submit(task Task) {
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
	p.taskQueue <- task

}

func (p *Pool) SubmitBefore(task Task, time time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), time)
	p.Submit(
		TaskFunc(func() {
			select {
			case <-ctx.Done():
				cancel()
			default:
				task.Process()
			}
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

		node := p.idleWorks.head
		for node != nil {

			if time.Now().Unix() > node.val.lastActiveAt.Unix()+1 {
				if p.idleWorks.head != p.idleWorks.tail {
					if node == p.idleWorks.head {
						p.idleWorks.head.next.prev = nil
						p.idleWorks.head = p.idleWorks.head.next

					} else if node == p.idleWorks.tail {
						p.idleWorks.tail.prev.next = nil
						p.idleWorks.tail = p.idleWorks.tail.prev

					} else {
						node.next.prev = node.prev
						node.prev.next = node.next
					}
				} else {
					p.idleWorks.head = nil
					p.idleWorks.tail = nil
				}

			}

			node = node.next

		}

		p.muIdle.Unlock()
		runtime.Gosched()
	}
}

func (p *Pool) Close() {
	close(p.closePoolCn)
}

func (p *Pool) GetRunningWorkersNum() int64 {
	return atomic.LoadInt64(&p.runningWorkersNum)
}
