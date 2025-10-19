package agilepool

import (
	"log"
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
		task.Process()
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
				log.Println("taskQueue closed,exiting")
				return
			}

			if task == nil {
				log.Println("nil task received, exiting")
				return
			}
			w.lastActiveAt = time.Now()
			task.Process()

		default:
			w.pool.addToIdle(w)
			break loop

		}
	}

}
