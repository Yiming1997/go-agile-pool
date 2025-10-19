package agilepool_test

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
)

const (
	workerCount = 10000
	taskCount   = 10000000
)

const (
	RunTimes           = 1e7
	PoolCap            = 5e4
	BenchParam         = 10
	DefaultExpiredTime = 10 * time.Second
	burstSize          = 10000
	FibDepth           = 100000
)

func cpuIntensiveTask(maxNumber int) int {
	count := 0
	for n := 2; n <= maxNumber; n++ {
		if isPrime(n) {
			count++
		}
	}
	return count
}

func isPrime(n int) bool {
	if n <= 1 {
		return false
	}
	if n == 2 {
		return true
	}
	if n%2 == 0 {
		return false
	}

	sqrt := int(math.Sqrt(float64(n)))
	for i := 3; i <= sqrt; i += 2 {
		if n%i == 0 {
			return false
		}
	}
	return true
}

func demoFunc(n int) {
	result := fibonacci(n)

	if result == 0 {
		fmt.Println("Impossible")
	}
}

func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}
func BenchmarkGoroutines(b *testing.B) {
	pool := agilepool.NewPool()
	pool.Init()

	wait := sync.WaitGroup{}
	for i := 0; i < b.N; i++ {
		for j := 0; j < RunTimes; j++ {
			wait.Add(1)
			go func() {
				pool.Submit(func() {
					defer wait.Done()
					time.Sleep(10 * time.Millisecond)

				})
			}()
		}
	}

	wait.Wait()

}
func BenchmarkEfficientPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool()
		pool.InitConfig().WithCleanPeriod(500 * time.Millisecond).WithTaskQueueSize(10000).WithWorkerNumCapacity(20000)
		pool.Init()

		var wg sync.WaitGroup
		wg.Add(taskCount)

		for j := 0; j < taskCount; j++ {
			go func() {
				pool.Submit(func() {
					time.Sleep(10 * time.Millisecond)

					wg.Done()
				})
			}()

		}

		wg.Wait()
		pool.Close()
	}
}
