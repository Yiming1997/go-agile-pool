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

	// NOTE: workerPool.Put(w) is intentionally called only on the "terminal"
	// exit paths (queue closed / nil task) below, NOT in a defer. Putting w to
	// the sync.Pool when the worker has just been added to idleWorks would
	// place the same *worker pointer in two containers at once; subsequent
	// Submits could then concurrently spawn two goroutines on the same
	// *worker via Pop and workerPool.Get respectively, causing a data race
	// on w.lastActiveAt and phantom duplicates in idleWorks.

loop:
	for {
		select {
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				w.pool.logger.Println("taskQueue closed,exiting")
				w.pool.addRunningWorkersNum(-1)
				w.pool.workerPool.Put(w)
				return
			}

			if task == nil {
				w.pool.logger.Println("nil task received, exiting")
				w.pool.addRunningWorkersNum(-1)
				w.pool.workerPool.Put(w)
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
					w.pool.workerPool.Put(w)
					return
				}
				if task == nil {
					w.pool.logger.Println("nil task received, exiting")
					w.pool.addRunningWorkersNum(-1)
					w.pool.workerPool.Put(w)
					return
				}
				w.lastActiveAt = time.Now()
				w.runTask(task)
			default:
				// w is being parked in idleWorks; do NOT also put it in
				// workerPool.sync.Pool — see the note at the top of run().
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
