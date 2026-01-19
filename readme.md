## Introduction
goAgilePool is a lightweight goroutine pool for Golang, designed for simplicity and high performance
## Features
1. Customizable goroutine pool size
2. Configurable task queue size
3. Task timeout control
4. Automatic cleanup of idle workers upon timeout
5. Efficient worker reuse through FIFO worker queue management

## Installation
go get github.com/Yiming1997/go-agile-pool

## Usage
**Pool.Submit()**
```go
    //Initialize a pool
	pool := agilepool.NewPool()

    //set pool configuration with chained calls
	pool.InitConfig().
		WithCleanPeriod(500 * time.Millisecond).
		WithTaskQueueSize(10000).
		WithWorkerNumCapacity(20000)

	pool.Init()
	//submit tasks
	for i := 0; i < 20000000; i++ {
		go func() {
			pool.Submit(agilepool.TaskFunc(func() {
				time.Sleep(10 * time.Millisecond)
				return nil
			}))

		}()
	}
	//wait for all tasks to be done
	pool.Wait() 
```

**Pool.SubmitBefore()**    
go-agile-pool allows us to submit a task that must be executed before a specified deadline,otherwise it will be canceled
```go
	agilePool.SubmitBefore(
				agilepool.TaskFunc(func() error {
					time.Sleep(10 * time.Millisecond)
					return nil
				}), 10*time.Second,
			)

```

**benchmark**   
Run this benchmark test，and we will see how fast the pool processes its tasks.
```go

const (
	taskCount = 10000000
)


func BenchmarkAgilePool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := agilepool.NewPool()
		pool.InitConfig().WithCleanPeriod(500 * time.Millisecond).WithTaskQueueSize(10000).WithWorkerNumCapacity(20000)
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

```

```
BenchmarkAgilePool-14    	       1	5881506800 ns/op	230601408 B/op	10871762 allocs/op
```