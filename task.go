package agilepool

import (
	"math"
	"time"
)

type Task interface {
	Process()
}

type TaskFunc func() error

func (tf TaskFunc) Process() {
	tf()
}

type TaskWithRetry struct {
	MinBackOff      time.Duration
	MaxBackOff      time.Duration
	AttemptNum      uint
	BackOffStrategy func(min, max time.Duration, attemptNum uint) time.Duration
	Task            func() error
}

func (t *TaskWithRetry) Process() {
	if t.Task() != nil {
		t.runBackOffStrategy()
	}
}

func (t *TaskWithRetry) runBackOffStrategy() {
	for i := 1; i <= int(t.AttemptNum); i++ {
		backOffTime := t.getBackOffTime(uint(i))
		time.Sleep(backOffTime)
		if t.Task() != nil {
			continue
		}
		break
	}
}

func (t *TaskWithRetry) getBackOffTime(attemptNum uint) time.Duration {
	if t.BackOffStrategy != nil {
		return t.BackOffStrategy(t.MinBackOff, t.MaxBackOff, t.AttemptNum)
	}

	return defaultBackOffStrategy(t.MinBackOff, t.MaxBackOff, attemptNum)
}

func defaultBackOffStrategy(min, max time.Duration, attemptNum uint) time.Duration {
	mult := math.Pow(2, float64(attemptNum)) * float64(min)
	sleep := time.Duration(mult)
	if float64(sleep) != mult || sleep > max {
		sleep = max
	}
	return sleep
}
