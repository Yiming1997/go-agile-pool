package agilepool

import (
	"math"
	"time"
)

type Task interface {
	Process()
}

// TaskFunc wraps a func() error into a Task. Note: the returned error is
// silently ignored — if you need error handling or retries, use TaskWithRetry.
type TaskFunc func() error

func (tf TaskFunc) Process() {
	tf()
}

type TaskWithRetry struct {
	MinBackOff      time.Duration
	MaxBackOff      time.Duration
	RetryNum        uint
	BackOffStrategy func(min, max time.Duration, retryNum uint) time.Duration
	Task            func() error
	// Pool must be set before calling Process so that retries can be
	// re-submitted to the pool instead of blocking the current worker.
	Pool *Pool
}

func (t *TaskWithRetry) Process() {
	if t.Task() == nil {
		return
	}
	// First failure: re-submit a retry task to the pool instead of sleeping.
	t.reSubmitRetry(1)
}

// reSubmitRetry creates a delayed retry and submits it back to the pool.
// The delay is achieved via time.AfterFunc, which schedules the re-submission
// after the backoff period without blocking any worker.
func (t *TaskWithRetry) reSubmitRetry(attempt uint) {
	if attempt > t.RetryNum {
		return
	}
	backOff := t.getBackOffTime(attempt)
	time.AfterFunc(backOff, func() {
		if t.Pool == nil {
			// Fallback: if no pool is set, run inline (blocking).
			t.runRetryInline(attempt)
			return
		}
		t.Pool.Submit(&retryTask{
			parent:  t,
			attempt: attempt,
		})
	})
}

// retryTask is the internal Task submitted for each retry attempt.
type retryTask struct {
	parent  *TaskWithRetry
	attempt uint
}

func (rt *retryTask) Process() {
	if rt.parent.Task() == nil {
		return
	}
	rt.parent.reSubmitRetry(rt.attempt + 1)
}

// runRetryInline is a fallback for when Pool is not set.
func (t *TaskWithRetry) runRetryInline(attempt uint) {
	for i := attempt; i <= t.RetryNum; i++ {
		backOffTime := t.getBackOffTime(i)
		time.Sleep(backOffTime)
		if t.Task() == nil {
			break
		}
	}
}

func (t *TaskWithRetry) getBackOffTime(retryNum uint) time.Duration {
	if t.BackOffStrategy != nil {
		return t.BackOffStrategy(t.MinBackOff, t.MaxBackOff, retryNum)
	}

	return defaultBackOffStrategy(t.MinBackOff, t.MaxBackOff, retryNum)
}

func defaultBackOffStrategy(min, max time.Duration, retryNum uint) time.Duration {
	mult := math.Pow(2, float64(retryNum)) * float64(min)
	sleep := time.Duration(mult)
	if float64(sleep) != mult || sleep > max {
		sleep = max
	}
	return sleep
}