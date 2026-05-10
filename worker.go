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
		func() {
			defer func() {
				if p := recover(); p != nil {
					w.pool.logger.Printf("worker exits from panic: %v\n%s\n", p, debug.Stack())
				}
			}()
			defer w.pool.wg.Done()
			task.Process()
		}()

	}

	defer func() {
		w.pool.addRunningWorkersNum(-1)
		w.pool.workerPool.Put(w)
	}()

loop:
	for {
		select {
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				w.pool.logger.Println("taskQueue closed,exiting")
				return
			}

			if task == nil {
				w.pool.logger.Println("nil task received, exiting")
				return
			}
			w.lastActiveAt = time.Now()
			func() {
				defer func() {
					if p := recover(); p != nil {
						w.pool.logger.Printf("worker exits from panic: %v\n%s\n", p, debug.Stack())
					}
				}()
				defer w.pool.wg.Done()
				task.Process()
			}()

		default:
			w.pool.addToIdle(w)
			break loop

		}
	}

}
