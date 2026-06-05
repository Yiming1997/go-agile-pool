package agilepool

import (
	"errors"
	"testing"
	"time"
)

func TestTaskFuncProcessCallsFunc(t *testing.T) {
	called := false
	tf := TaskFunc(func() error {
		called = true
		return nil
	})
	tf.Process()
	if !called {
		t.Fatal("TaskFunc.Process() did not call the underlying func")
	}
}

// TestTaskWithRetryInvocationCount verifies the task is invoked RetryNum+1
// times when it always errors (1 initial in Process + RetryNum in the retry
// loop). Backoff is nanoseconds so the test stays fast.
func TestTaskWithRetryInvocationCount(t *testing.T) {
	var calls int
	task := &TaskWithRetry{
		MinBackOff: 1 * time.Nanosecond,
		MaxBackOff: 10 * time.Nanosecond,
		RetryNum:   3,
		Task: func() error {
			calls++
			return errors.New("always fails")
		},
	}
	task.Process()
	if calls != 4 {
		t.Fatalf("task invoked %d times, want 4 (1 + RetryNum=3)", calls)
	}
}

func TestTaskWithRetryStopsOnSuccess(t *testing.T) {
	var calls int
	task := &TaskWithRetry{
		MinBackOff: 1 * time.Nanosecond,
		MaxBackOff: 10 * time.Nanosecond,
		RetryNum:   5,
		Task: func() error {
			calls++
			return nil // succeeds on first try
		},
	}
	task.Process()
	if calls != 1 {
		t.Fatalf("task invoked %d times, want 1 (success on first try)", calls)
	}
}

// TestTaskWithRetryCustomBackOffUsed verifies a non-nil BackOffStrategy is used
// instead of the default, once per retry iteration. The strategy returns 0 so
// there is no real sleep.
func TestTaskWithRetryCustomBackOffUsed(t *testing.T) {
	var strategyCalls int
	task := &TaskWithRetry{
		MinBackOff: 1 * time.Second, // ignored: custom strategy returns 0
		MaxBackOff: 2 * time.Second,
		RetryNum:   2,
		BackOffStrategy: func(min, max time.Duration, retryNum uint) time.Duration {
			strategyCalls++
			return 0
		},
		Task: func() error {
			return errors.New("fail")
		},
	}
	task.Process()
	if strategyCalls != 2 {
		t.Fatalf("custom BackOffStrategy called %d times, want 2 (once per retry)", strategyCalls)
	}
}

// TestDefaultBackOffStrategyExponential verifies 2^retryNum * min below the cap.
func TestDefaultBackOffStrategyExponential(t *testing.T) {
	min := 1 * time.Millisecond
	max := 1 * time.Hour
	cases := []struct {
		retryNum uint
		want     time.Duration
	}{
		{1, 2 * time.Millisecond},
		{2, 4 * time.Millisecond},
		{3, 8 * time.Millisecond},
	}
	for _, tc := range cases {
		if got := defaultBackOffStrategy(min, max, tc.retryNum); got != tc.want {
			t.Errorf("defaultBackOffStrategy(min, max, %d) = %v, want %v", tc.retryNum, got, tc.want)
		}
	}
}

// TestDefaultBackOffStrategyCaps verifies the result is capped at max when the
// exponential value exceeds it, including the float-overflow path.
func TestDefaultBackOffStrategyCaps(t *testing.T) {
	min := 1 * time.Second
	max := 5 * time.Second
	if got := defaultBackOffStrategy(min, max, 10); got != max { // 1024s -> capped
		t.Errorf("defaultBackOffStrategy capped = %v, want %v", got, max)
	}
	if got := defaultBackOffStrategy(min, max, 1024); got != max { // overflow -> capped
		t.Errorf("defaultBackOffStrategy overflow = %v, want %v", got, max)
	}
}
