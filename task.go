package agilepool

import (
	"math"
	"time"
)

type Task interface {
	Process()
	BackOff(min, max time.Duration, attemptNum int) time.Duration
}

type TaskFunc func() error

func (tf TaskFunc) Process() {
	tf()
}

func (tf TaskFunc) BackOff(min, max time.Duration, attemptNum int) time.Duration {
	return 0
}

type TaskWithRetry struct {
	MinBackOff time.Duration
	MaxBackOff time.Duration
	AttemptNum int
	BackOff    func(min, max time.Duration, attemptNum int) time.Duration
	Task       func() error
}

func (t *TaskWithRetry) Process() {

}

func (t *TaskWithRetry) runBackOff() time.Duration {
	if t.BackOff != nil {
		return t.BackOff(t.MinBackOff, t.MaxBackOff, t.AttemptNum)
	}
	// 默认实现
	return defaultBackOff(t.MinBackOff, t.MaxBackOff, t.AttemptNum)
}

func defaultBackOff(min, max time.Duration, attemptNum int) time.Duration {
	mult := math.Pow(2, float64(attemptNum)) * float64(min)
	sleep := time.Duration(mult)
	if float64(sleep) != mult || sleep > max {
		sleep = max
	}
	return sleep
}
