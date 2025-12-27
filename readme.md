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
	pool := agilepool.NewPool()

	pool.InitConfig().
		WithCleanPeriod(500 * time.Millisecond).
		WithTaskQueueSize(10000).
		WithWorkerNumCapacity(20000)

	pool.Init()

	for i := 0; i < 20000000; i++ {
		go func() {
			pool.Submit(agilepool.TaskFunc(func() {
				time.Sleep(10 * time.Millisecond)
				return nil
			}))

		}()
	}

	pool.Wait() 
```

**Pool.SubmitBefore()**    
```go
	agilePool.SubmitBefore(
				agilepool.TaskFunc(func() error {
					time.Sleep(10 * time.Millisecond)
					return nil
				}), 10*time.Second,
			)

```
**benchmark**    
```
BenchmarkAgilePool-14    	       1	5881506800 ns/op	230601408 B/op	10871762 allocs/op
```