package agilepool_test

import (
	"sync"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
)

const (
	taskCount = 10000000
)

func BenchmarkAgilePoolMinHeap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool()
		// 20k worker capacity gives the best performance
		pool.InitConfig().WithCleanPeriod(500 * time.Millisecond).WithTaskQueueSize(10000).WithWorkerNumCapacity(20000).WithIdleContainerType(agilepool.MinHeapType)
		pool.Init()

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
		pool := agilepool.NewPool()
		// 20k worker capacity gives the best performance
		pool.InitConfig().WithCleanPeriod(500 * time.Millisecond).WithTaskQueueSize(10000).WithWorkerNumCapacity(20000).WithIdleContainerType(agilepool.LinkedListType)
		pool.Init()

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
		pool := agilepool.NewPool()
		// 20k worker capacity gives the best performance
		pool.InitConfig().WithCleanPeriod(500 * time.Millisecond).WithTaskQueueSize(10000).WithWorkerNumCapacity(20000).WithIdleContainerType(agilepool.MinHeapType)
		pool.Init()

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
		pool := agilepool.NewPool()
		// 20k worker capacity gives the best performance
		pool.InitConfig().WithCleanPeriod(500 * time.Millisecond).WithTaskQueueSize(10000).WithWorkerNumCapacity(20000).WithIdleContainerType(agilepool.LinkedListType)
		pool.Init()

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

func BenchmarkNativeGoroutine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 20000)

		for j := 0; j < taskCount; j++ {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				time.Sleep(10 * time.Millisecond)
			}()
		}
		wg.Wait()
	}
}