package agilepool_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
	"github.com/panjf2000/ants/v2"
)

const (
	taskCount  = 10000000
	queueSize  = 10000
	workerCap  = 20000
	clean      = 500 * time.Millisecond
)

// ioTask simulates an IO-blocking operation.
func ioTask() {
	time.Sleep(10 * time.Millisecond)
}

// ioPoolTask is the TaskFunc wrapper for ioTask.
func ioPoolTask() error {
	ioTask()
	return nil
}

// ---------------------------------------------------------------------------
// AgilePool benchmarks
// ---------------------------------------------------------------------------

func BenchmarkAgilePoolMinHeap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithCleanPeriod(clean),
			agilepool.WithTaskQueueSize(queueSize),
			agilepool.WithWorkerNumCapacity(workerCap),
			agilepool.WithIdleContainerType(agilepool.MinHeapType),
		))

		for j := 0; j < taskCount; j++ {
			go func() {
				pool.Submit(agilepool.TaskFunc(ioPoolTask))
			}()
		}
		pool.Wait()
		pool.Close()
	}
}

func BenchmarkAgilePoolLinkedList(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithCleanPeriod(clean),
			agilepool.WithTaskQueueSize(queueSize),
			agilepool.WithWorkerNumCapacity(workerCap),
			agilepool.WithIdleContainerType(agilepool.LinkedListType),
		))

		for j := 0; j < taskCount; j++ {
			go func() {
				pool.Submit(agilepool.TaskFunc(ioPoolTask))
			}()
		}
		pool.Wait()
		pool.Close()
	}
}

func BenchmarkAgilePoolSequentialMinHeap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithCleanPeriod(clean),
			agilepool.WithTaskQueueSize(queueSize),
			agilepool.WithWorkerNumCapacity(workerCap),
			agilepool.WithIdleContainerType(agilepool.MinHeapType),
		))

		for j := 0; j < taskCount; j++ {
			pool.Submit(agilepool.TaskFunc(ioPoolTask))
		}
		pool.Wait()
		pool.Close()
	}
}

func BenchmarkAgilePoolSequentialLinkedList(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithCleanPeriod(clean),
			agilepool.WithTaskQueueSize(queueSize),
			agilepool.WithWorkerNumCapacity(workerCap),
			agilepool.WithIdleContainerType(agilepool.LinkedListType),
		))

		for j := 0; j < taskCount; j++ {
			pool.Submit(agilepool.TaskFunc(ioPoolTask))
		}
		pool.Wait()
		pool.Close()
	}
}

// ---------------------------------------------------------------------------
// Comparison benchmarks
// ---------------------------------------------------------------------------

func BenchmarkAntsPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		p, _ := ants.NewPool(workerCap)

		for j := 0; j < taskCount; j++ {
			go func() {
				_ = p.Submit(func() {
					ioTask()
				})
			}()
		}
		p.Release()
	}
}

func BenchmarkNativeGoroutine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		sem := make(chan struct{}, workerCap)

		for j := 0; j < taskCount; j++ {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				ioTask()
			}()
		}
		wg.Wait()
	}
}

// ---------------------------------------------------------------------------
// Compare: run all implementations in one go
// ---------------------------------------------------------------------------

func BenchmarkCompare(b *testing.B) {
	b.Run("AgilePool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var m0, m1 runtime.MemStats
			runtime.ReadMemStats(&m0)

			pool := agilepool.NewPool(agilepool.NewConfig(
				agilepool.WithCleanPeriod(clean),
				agilepool.WithTaskQueueSize(queueSize),
				agilepool.WithWorkerNumCapacity(workerCap),
			))

			for j := 0; j < taskCount; j++ {
				pool.Submit(agilepool.TaskFunc(ioPoolTask))
			}
			pool.Wait()
			workers := pool.GetWorkerCreateCount()
			pool.Close()

			runtime.ReadMemStats(&m1)
			b.ReportMetric(float64(workers), "workers/op")
			b.ReportMetric(float64(m1.TotalAlloc-m0.TotalAlloc), "total_B/op")
		}
	})

	b.Run("Ants", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var m0, m1 runtime.MemStats
			runtime.ReadMemStats(&m0)

			p, _ := ants.NewPool(workerCap)

			for j := 0; j < taskCount; j++ {
				_ = p.Submit(func() {
					ioTask()
				})
			}
			p.Release()

			runtime.ReadMemStats(&m1)
			b.ReportMetric(float64(m1.TotalAlloc-m0.TotalAlloc), "total_B/op")
		}
	})

	b.Run("Native", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var m0, m1 runtime.MemStats
			runtime.ReadMemStats(&m0)

			var wg sync.WaitGroup
			sem := make(chan struct{}, workerCap)

			for j := 0; j < taskCount; j++ {
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					ioTask()
				}()
			}
			wg.Wait()

			runtime.ReadMemStats(&m1)
			b.ReportMetric(float64(m1.TotalAlloc-m0.TotalAlloc), "total_B/op")
		}
	})
}
