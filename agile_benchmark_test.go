package agilepool_test

import (
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
)

const (
	taskCount = 10000000
)

func BenchmarkAgilePoolMinHeap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// 20k worker capacity gives the best performance

		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithCleanPeriod(500*time.Millisecond),
			agilepool.WithTaskQueueSize(10000),
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

func BenchmarkAgilePoolLinkedList(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// 20k worker capacity gives the best performance
		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithCleanPeriod(500*time.Millisecond),
			agilepool.WithTaskQueueSize(10000),
			agilepool.WithWorkerNumCapacity(20000),
			agilepool.WithIdleContainerType(agilepool.LinkedListType),
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

func BenchmarkAgilePoolSequentialMinHeap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// 20k worker capacity gives the best performance
		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithCleanPeriod(500*time.Millisecond),
			agilepool.WithTaskQueueSize(10000),
			agilepool.WithWorkerNumCapacity(20000),
			agilepool.WithIdleContainerType(agilepool.MinHeapType),
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

func BenchmarkAgilePoolSequentialLinkedList(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// 20k worker capacity gives the best performance
		pool := agilepool.NewPool(agilepool.NewConfig(
			agilepool.WithCleanPeriod(500*time.Millisecond),
			agilepool.WithTaskQueueSize(10000),
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
