package agilepool_test

import (
	agilepool "agilePool"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"
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
	FibDepth           = 100000 // 斐波那契计算深度，控制计算强度
)

// func demoFunc(n int) {

// 	a, b := 0, 1
// 	for i := 0; i < n; i++ {
// 		a, b = b, a+b
// 	}

// 	// 添加伪结果使用防止编译器优化
// 	if b == 0 {
// 		fmt.Println("Impossible")
// 	}

// }

func cpuIntensiveTask(maxNumber int) int {
	count := 0
	for n := 2; n <= maxNumber; n++ {
		if isPrime(n) {
			count++
		}
	}
	return count
}

// 判断一个数是否为质数
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

	// 添加伪结果使用防止编译器优化
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
		pool.InitConfig().WithCleanPeriod(500 * time.Millisecond).WithTaskQueueSize(10000).WithWorkerNumCapacity(10000)
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

			// pool.Submit(func() {
			// 	time.Sleep(10 * time.Millisecond)

			// 	wg.Done()
			// })

		}

		wg.Wait()
		pool.Close()
	}
}
