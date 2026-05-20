package agilepool

import (
	"runtime/debug"
	"time"
)

type worker struct {
	pool         *Pool
	lastActiveAt time.Time
}

func newWorker(p *Pool) *worker {
	w := &worker{
		pool: p,
	}
	return w
}
func (w *worker) run(task Task) {
	w.lastActiveAt = time.Now()
	if task != nil {
		w.runTask(task)
	}

	defer w.pool.workerPool.Put(w)

loop:
	for {
		select {
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				w.pool.logger.Println("taskQueue closed,exiting")
				w.pool.addRunningWorkersNum(-1)
				return
			}

			if task == nil {
				w.pool.logger.Println("nil task received, exiting")
				w.pool.addRunningWorkersNum(-1)
				return
			}
			w.lastActiveAt = time.Now()
			w.runTask(task)

		default:
			// Acquire pool lock and re-check the queue atomically.
			// This serializes the "decide to exit" decision with Submit's slow-path
			// push, so a task pushed concurrently cannot be stranded in the queue
			// after this worker decrements runningWorkersNum.
			w.pool.lock.Lock()
			select {
			case task, ok := <-w.pool.taskQueue:
				w.pool.lock.Unlock()
				if !ok {
					w.pool.logger.Println("taskQueue closed,exiting")
					w.pool.addRunningWorkersNum(-1)
					return
				}
				if task == nil {
					w.pool.logger.Println("nil task received, exiting")
					w.pool.addRunningWorkersNum(-1)
					return
				}
				w.lastActiveAt = time.Now()
				w.runTask(task)
			default:
				w.pool.addToIdle(w)
				w.pool.addRunningWorkersNum(-1)
				w.pool.lock.Unlock()
				break loop
			}
		}
	}
}

func (w *worker) runTask(task Task) {
	defer func() {
		if p := recover(); p != nil {
			w.pool.logger.Printf("worker exits from panic: %v\n%s\n", p, debug.Stack())
		}
	}()
	defer w.pool.wg.Done()
	task.Process()
}
