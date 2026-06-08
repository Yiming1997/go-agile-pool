# go-agile-pool

[简体中文](README.zh-CN.md)

`go-agile-pool` is a lightweight goroutine pool for Go. It provides bounded worker concurrency, a buffered task queue, idle worker reuse, retryable tasks, and graceful shutdown helpers for applications that need to submit large numbers of small asynchronous jobs without creating unbounded goroutines.

## Features

- Bounded worker count with configurable capacity.
- Buffered task queue with blocking and non-blocking submit modes.
- Automatic idle worker cleanup.
- Pluggable idle worker containers: FIFO linked list or min-heap ordered by last active time.
- Retryable tasks with exponential backoff or a custom backoff strategy.
- `Wait` and `Close` helpers for graceful shutdown.
- Custom logger support for integrating pool logs into your application logger.

## Installation

```bash
go get github.com/Yiming1997/go-agile-pool
```

## Quick Start

```go
package main

import (
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
)

func main() {
	pool := agilepool.NewPool(agilepool.NewConfig())
	defer pool.Close()

	for i := 0; i < 1000; i++ {
		pool.Submit(agilepool.TaskFunc(func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		}))
	}

	pool.Wait()
}
```

## Configuration

Create a pool with `NewConfig` and pass options for the behavior you want:

```go
pool := agilepool.NewPool(agilepool.NewConfig(
	agilepool.WithCleanPeriod(500*time.Millisecond),
	agilepool.WithTaskQueueSize(10000),
	agilepool.WithWorkerNumCapacity(20000),
	agilepool.WithBlockMode(agilepool.BLOCK),
	agilepool.WithIdleContainerType(agilepool.MinHeapType),
))
defer pool.Close()
```

Available options:

| Option | Default | Description |
| --- | --- | --- |
| `WithCleanPeriod` | `500ms` | How often the background cleaner checks idle workers. |
| `WithTaskQueueSize` | `10000` | Buffered queue size used when all workers are busy. |
| `WithWorkerNumCapacity` | `math.MaxInt64` | Maximum number of running workers. |
| `WithBlockMode` | `BLOCK` | `BLOCK` queues tasks when capacity is reached; `NONBLOCK` drops submissions when capacity is reached. |
| `WithIdleContainerType` | `LinkedListType` | Data structure used to store idle workers. |

## Submitting Tasks

Use `Submit` for normal asynchronous execution:

```go
pool.Submit(agilepool.TaskFunc(func() error {
	// Do work here.
	return nil
}))
```

`TaskFunc` returns an error for compatibility with retryable task patterns, but plain `Submit` does not inspect the returned error. Use `TaskWithRetry` when failures should trigger retries.

## Submit Before a Deadline

`SubmitBefore` schedules a task with a timeout window. If the timeout has already expired before the worker starts the task, the task is skipped.

```go
pool.SubmitBefore(
	agilepool.TaskFunc(func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}),
	10*time.Second,
)
```

## Retryable Tasks

`TaskWithRetry` retries a task when it returns an error. By default it uses exponential backoff between `MinBackOff` and `MaxBackOff`; you can also provide `BackOffStrategy`.

```go
pool.Submit(&agilepool.TaskWithRetry{
	MinBackOff: 1 * time.Second,
	MaxBackOff: 30 * time.Second,
	RetryNum:   3,
	Task: func() error {
		return errors.New("temporary failure")
	},
})
```

## Idle Worker Containers

The pool reuses idle workers. You can choose how idle workers are stored:

| Container | Ordering | Pop behavior | Expiration cleanup | Typical use |
| --- | --- | --- | --- | --- |
| `LinkedListType` | Insertion order | Reuses the first idle worker | Full traversal, `O(n)` | General purpose, simple FIFO reuse. |
| `MinHeapType` | `lastActiveAt` | Reuses the least recently active worker | Stops once the oldest active worker is not expired, `O(k log n)` | Efficient cleanup when many workers become idle. |

```go
pool := agilepool.NewPool(agilepool.NewConfig(
	agilepool.WithIdleContainerType(agilepool.MinHeapType),
	agilepool.WithWorkerNumCapacity(20000),
))
defer pool.Close()
```

## Lifecycle

Call `Wait` to block until submitted in-flight tasks complete:

```go
pool.Wait()
```

Call `Close` when the pool is no longer needed. `Close` is idempotent. After it is called, new submissions are ignored, while already submitted tasks can still finish and be waited on.

```go
pool.Close()
```

## Custom Logger

By default the pool uses the standard library logger. Replace it with any logger that implements `Printf` and `Println`:

```go
pool.SetLogger(log.Default())
```

## Tests and Benchmarks

Run the test suite:

```bash
go test ./...
```

Run all benchmarks:

```bash
go test -bench=. -benchtime=1x -timeout=2h -run=^$ -count=1
```

Run a single benchmark:

```bash
go test -bench=BenchmarkAgilePoolMinHeap -benchtime=1x -timeout=2h -run=^$ -count=1
```

The benchmark suite compares concurrent and sequential submissions with both idle container implementations, plus a native goroutine baseline.
