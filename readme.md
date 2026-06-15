# agilePool

<p align="center">
  <img src="assets/logo.jpg" alt="go-agile-pool logo" width="260">
</p>

<p align="center">
  <a href="https://github.com/Yiming1997/agilePool/actions/workflows/ci.yml"><img src="https://github.com/Yiming1997/agilePool/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-%3E%3D1.23.4-00ADD8" alt="Go Version"></a>
  <a href="https://github.com/Yiming1997/agilePool/tags"><img src="https://img.shields.io/github/v/tag/Yiming1997/go-agile-pool?label=tag" alt="Tag"></a>
  <a href="https://pkg.go.dev/github.com/Yiming1997/agilePool"><img src="https://pkg.go.dev/badge/github.com/Yiming1997/agilePool.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/Yiming1997/agilePool"><img src="https://goreportcard.com/badge/github.com/Yiming1997/agilePool" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/Yiming1997/go-agile-pool" alt="License"></a>
</p>

[简体中文](README.zh-CN.md)

`agilePool` is a lightweight goroutine pool for Go. It provides bounded worker concurrency, a buffered task queue, idle worker reuse, retryable tasks, and graceful shutdown helpers for applications that need to submit large numbers of small asynchronous jobs without creating unbounded goroutines.

## Features

- Bounded worker count with configurable capacity.
- Buffered task queue with blocking and non-blocking submit modes.
- Automatic idle worker cleanup.
- Pluggable idle worker containers: FIFO linked list or min-heap ordered by last active time.
- Retryable tasks with exponential backoff or a custom backoff strategy.
- Context-aware submission and cooperative cancellation for running tasks.
- `Wait` and `Close` helpers for graceful shutdown.
- Custom logger support for integrating pool logs into your application logger.

## Installation

```bash
go get github.com/Yiming1997/agilePool
```

## Quick Start

```go
package main

import (
	"time"

	agilepool "github.com/Yiming1997/agilePool"
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

## Submit With Context

Use `SubmitCtx` when a task needs cancellation support, and check the same `ctx` inside the task closure:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

pool.SubmitCtx(ctx, agilepool.TaskFunc(func() error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Do work here.
		return nil
	}
}))
```

`SubmitCtx` cancellation behavior:

- If `ctx` is already canceled before submission, the task is not accepted.
- In `BLOCK` mode, if the queue is full, submission waits for queue space; cancellation while waiting drops the submission.
- If the task has already been queued but has not started, the worker checks `ctx` after dequeue and skips execution when it is canceled.
- If the task has already started, the pool does not forcibly stop the goroutine; the task must check `ctx.Done()` itself or pass `ctx` to downstream HTTP, database, RPC, or similar calls.

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

The following benchmark code only tests go-agile-pool. Save it as a `_test.go` file and run:

```bash
go test -bench=BenchmarkAgilePool -benchtime=1x -timeout=2h -run=^$
```

```go
package agilepool_test

import (
	"testing"
	"time"

	agilepool "github.com/Yiming1997/agilePool"
)

const taskCount = 10000000

// Concurrent submission benchmark
func BenchmarkAgilePoolMinHeap(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithWorkerNumCapacity(20000),
			agilepool.WithIdleContainerType(agilepool.MinHeapType),
		))

		for j := 0; j < taskCount; j++ {
			go func() {
				pool.Submit(agilepool.TaskFunc(func() error {
					time.Sleep(10 * time.Millisecond)
					return nil
				}))
			}()
		}
		pool.Wait()
		pool.Close()
	}
}

// Sequential submission benchmark
func BenchmarkAgilePoolSequentialLinkedList(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithWorkerNumCapacity(20000),
			agilepool.WithIdleContainerType(agilepool.LinkedListType),
		))

		for j := 0; j < taskCount; j++ {
			pool.Submit(agilepool.TaskFunc(func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
			}))
		}
		pool.Wait()
		pool.Close()
	}
}
```

> **Note**: Adjust `taskCount` for quick smoke tests (e.g. `1000`).

The full benchmark suite compares concurrent and sequential submissions with multiple idle container implementations, plus native goroutine and popular Goroutine pool libraries. All benchmarks simulate an IO-bound task with `time.Sleep(10 * time.Millisecond)` and run **10 million tasks** (worker capacity = 20,000, go 1.23, measured via `b.ReportAllocs()`).

**Concurrent submission (10M tasks):**

| Pool | Time | Memory | Allocations |
|------|------|--------|-------------|
| **AgilePool MinHeap** | 6.20s | 463.4 MB | 10,303,830 |
| **AgilePool LinkedList** | 6.95s | 419.5 MB | 10,202,989 |
| Native(sem) | 6.65s | 1,201.2 MB | 20,002,053 |
| Ants | 9.40s | 495.9 MB | 20,184,630 |
| Pond | 26.9s | 4,328.9 MB | 73,225,096 |
| Gowp | 9.80s | 2,185.1 MB | 20,219,299 |

AgilePool MinHeap is the **fastest** (6.20s) while also being the **most memory-efficient** (463.4 MB). Native(sem) is close in speed but uses 2.6× memory. Pond is the worst across all metrics.

**Sequential submission (10M tasks):**

| Pool | Time | Memory | Allocations |
|------|------|--------|-------------|
| **AgilePool Seq LinkedList** | 5.37s | 166.7 MB | 137,045 |
| **AgilePool Seq Slice** | 5.34s | 167.5 MB | 103,834 |
| **AgilePool Seq MinHeap** | 5.76s | 283.3 MB | 1,874,468 |
| Ants Seq | 7.87s | 171.5 MB | 10,140,321 |
| Pond Seq | 13.34s | 3,363.9 MB | 80,004,361 |
| Gowp Seq | 6.07s | 1,929.9 MB | 10,061,537 |
