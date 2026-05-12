package agilepool

import (
	"runtime/debug"
	"time"
)

type worker struct {
	pool         *Pool
	lastActiveAt time.Time
}

func (w *worker) run(task Task) {
	defer func() {
		w.pool.addRunningWorkersNum(-1)
		w.pool.workerPool.Put(w)
	}()

	w.lastActiveAt = time.Now()

	// Process the initial task. wg.Done() is always called regardless
	// of whether the task is nil or panics.
	func() {
		defer func() {
			if p := recover(); p != nil {
				w.pool.logger.Printf("worker exits from panic: %v\n%s\n", p, debug.Stack())
			}
		}()
		defer w.pool.wg.Done()
		if task != nil {
			task.Process()
		}
	}()

	// Keep pulling tasks from the queue. The worker stays alive until
	// the idle timer fires (expiryDuration) or the task queue is closed.
	idleTimer := time.NewTimer(w.pool.config.expiryDuration)
	defer idleTimer.Stop()

	for {
		idleTimer.Reset(w.pool.config.expiryDuration)
		select {
		case task, ok := <-w.pool.taskQueue:
			if !ok {
				// taskQueue closed, worker exits.
				return
			}

			if task == nil {
				w.pool.logger.Println("nil task received, exiting")
				return
			}
			idleTimer.Stop()
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

		case <-idleTimer.C:
			// Idle timeout: worker exits, freeing a worker slot.
			return
		}
	}
}