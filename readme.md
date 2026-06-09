# go-agile-pool

<p align="center">
  <img src="assets/logo.jpg" alt="go-agile-pool logo" width="260">
</p>

<p align="center">
  <a href="https://github.com/Yiming1997/go-agile-pool/actions/workflows/ci.yml"><img src="https://github.com/Yiming1997/go-agile-pool/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-%3E%3D1.23.4-00ADD8" alt="Go Version"></a>
  <a href="https://github.com/Yiming1997/go-agile-pool/tags"><img src="https://img.shields.io/github/v/tag/Yiming1997/go-agile-pool?label=tag" alt="Tag"></a>
  <a href="https://pkg.go.dev/github.com/Yiming1997/go-agile-pool"><img src="https://pkg.go.dev/badge/github.com/Yiming1997/go-agile-pool.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/Yiming1997/go-agile-pool"><img src="https://goreportcard.com/badge/github.com/Yiming1997/go-agile-pool" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/Yiming1997/go-agile-pool" alt="License"></a>
</p>

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

The benchmark suite compares concurrent and sequential submissions with multiple idle container implementations, plus native goroutine and popular Goroutine pool libraries. All benchmarks simulate an IO-bound task with `time.Sleep(10 * time.Millisecond)`.

Below are results at **worker capacity = 20,000** (go 1.23, measured via `-benchmem`):

| Pool | 100K tasks | 500K tasks | 1M tasks |
|------|-----------|-----------|---------|
| **AgilePool** | 163ms / 34.7 MB | 385ms / 46.2 MB | **711ms** / **51.3 MB** |
| Ants | 99ms / 10.8 MB | 433ms / 20.9 MB | 831ms / 32.8 MB |
| Pond | 156ms / 15.1 MB | 984ms / 23.3 MB | 2241ms / 39.3 MB |
| Gowp | 117ms / 25.2 MB | 483ms / 98.4 MB | 940ms / 193.3 MB |
| Native(sem) | **69ms** / 13.5 MB | **316ms** / 61.2 MB | 633ms / 120.4 MB |

> At 1M tasks, AgilePool is the **fastest** (711ms) and **most memory-efficient** (51.3 MB). Ants is close on memory (32.8 MB) but slower (831ms). Gowp and Native suffer from severe memory blow-up at scale (193 MB / 120 MB).

Memory efficiency ranking at 1M tasks: **AgilePool > Ants > Pond > Native > Gowp**.
