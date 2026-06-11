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

[English](readme.md)

`go-agile-pool` 是一个轻量级 Go goroutine 池。它提供有界 worker 并发、缓冲任务队列、空闲 worker 复用、可重试任务和优雅关闭能力，适合需要提交大量小型异步任务、同时又不希望无限制创建 goroutine 的应用。

## 特性

- 可限制 worker 数量，并支持自定义容量。
- 支持缓冲任务队列，以及阻塞和非阻塞提交模式。
- 自动清理超时空闲 worker。
- 可插拔空闲 worker 容器：FIFO 链表，或按最后活跃时间排序的最小堆。
- 支持带重试次数的任务，可使用指数退避或自定义退避策略。
- 提供 `Wait` 和 `Close`，便于优雅关闭。
- 支持自定义 logger，方便接入应用自己的日志系统。

## 安装

```bash
go get github.com/Yiming1997/go-agile-pool
```

## 快速开始

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

## 配置

使用 `NewConfig` 创建配置，并通过 option 调整行为：

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

可用配置项：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `WithCleanPeriod` | `500ms` | 后台清理器检查空闲 worker 的频率。 |
| `WithTaskQueueSize` | `10000` | 所有 worker 忙碌时使用的缓冲队列大小。 |
| `WithWorkerNumCapacity` | `math.MaxInt64` | 最大运行 worker 数量。 |
| `WithBlockMode` | `BLOCK` | `BLOCK` 在达到容量后将任务放入队列；`NONBLOCK` 在达到容量后丢弃提交。 |
| `WithIdleContainerType` | `LinkedListType` | 用于保存空闲 worker 的数据结构。 |

## 提交任务

普通异步任务使用 `Submit`：

```go
pool.Submit(agilepool.TaskFunc(func() error {
	// 在这里执行任务逻辑。
	return nil
}))
```

`TaskFunc` 返回 `error` 是为了兼容可重试任务模式，但普通 `Submit` 不会检查这个返回值。如果失败后需要自动重试，请使用 `TaskWithRetry`。

## 在截止时间前提交

`SubmitBefore` 会给任务设置一个超时时间窗口。如果 worker 开始执行任务前超时时间已经到达，该任务会被跳过。

```go
pool.SubmitBefore(
	agilepool.TaskFunc(func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}),
	10*time.Second,
)
```

## 可重试任务

`TaskWithRetry` 会在任务返回错误时重试。默认使用 `MinBackOff` 到 `MaxBackOff` 之间的指数退避，也可以通过 `BackOffStrategy` 提供自定义退避策略。

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

## 空闲 Worker 容器

池会复用空闲 worker。你可以选择空闲 worker 的存储方式：

| 容器 | 排序方式 | Pop 行为 | 过期清理 | 适用场景 |
| --- | --- | --- | --- | --- |
| `LinkedListType` | 插入顺序 | 复用第一个空闲 worker | 全量遍历，`O(n)` | 通用场景，简单 FIFO 复用。 |
| `MinHeapType` | `lastActiveAt` | 复用最久未活跃的 worker | 当最老的 worker 未过期时停止，`O(k log n)` | 大量 worker 空闲时更高效地清理过期 worker。 |

```go
pool := agilepool.NewPool(agilepool.NewConfig(
	agilepool.WithIdleContainerType(agilepool.MinHeapType),
	agilepool.WithWorkerNumCapacity(20000),
))
defer pool.Close()
```

## 生命周期

调用 `Wait` 阻塞等待已提交的运行中任务完成：

```go
pool.Wait()
```

当不再需要池时调用 `Close`。`Close` 是幂等的。调用后，新的提交会被忽略，已经提交的任务仍可以继续完成，并可通过 `Wait` 等待。

```go
pool.Close()
```

## 自定义 Logger

默认情况下，池使用标准库 logger。你可以替换为任何实现了 `Printf` 和 `Println` 的 logger：

```go
pool.SetLogger(log.Default())
```

## 测试与基准测试

运行测试：

```bash
go test ./...
```

运行全部基准测试：

```bash
go test -bench=. -benchtime=1x -timeout=2h -run=^$ -count=1
```

运行单个基准测试：

```bash
go test -bench=BenchmarkAgilePoolMinHeap -benchtime=1x -timeout=2h -run=^$ -count=1
```

以下基准测试代码仅测试 go-agile-pool 自身。保存为 `_test.go` 文件后运行：

```bash
go test -bench=BenchmarkAgilePool -benchtime=1x -timeout=2h -run=^$
```

```go
package agilepool_test

import (
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
)

const taskCount = 10000000

// 并发提交基准测试
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

// 顺序提交基准测试
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

> **提示**：可调整 `taskCount` 快速测试（如 `1000`）。

完整基准测试会比较并发提交、顺序提交、多种空闲容器实现，以及原生 goroutine 和流行的 goroutine 池库。所有基准测试模拟 IO 密集型任务，使用 `time.Sleep(10 * time.Millisecond)`，运行 **1000 万任务**（worker 容量 = 20,000，go 1.23，通过 `b.ReportAllocs()` 测量）。

**并发提交（1000 万任务）：**

| 库 | 耗时 | 内存 | 分配次数 |
|------|------|--------|---------|
| **AgilePool MinHeap** | 6.20s | 463.4 MB | 10,303,830 |
| **AgilePool LinkedList** | 6.95s | 419.5 MB | 10,202,989 |
| Native(sem) | 6.65s | 1,201.2 MB | 20,002,053 |
| Ants | 9.40s | 495.9 MB | 20,184,630 |
| Pond | 26.9s | 4,328.9 MB | 73,225,096 |
| Gowp | 9.80s | 2,185.1 MB | 20,219,299 |

AgilePool MinHeap **速度最快**（6.20s）且 **内存最优**（463.4 MB）。Native(sem) 速度接近但内存是其 2.6 倍。Pond 在所有指标中表现最差。

**顺序提交（1000 万任务）：**

| 库 | 耗时 | 内存 | 分配次数 |
|------|------|--------|---------|
| **AgilePool Seq LinkedList** | 5.37s | 166.7 MB | 137,045 |
| **AgilePool Seq Slice** | 5.34s | 167.5 MB | 103,834 |
| **AgilePool Seq MinHeap** | 5.76s | 283.3 MB | 1,874,468 |
| Ants Seq | 7.87s | 171.5 MB | 10,140,321 |
| Pond Seq | 13.34s | 3,363.9 MB | 80,004,361 |
| Gowp Seq | 6.07s | 1,929.9 MB | 10,061,537 |

