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

    // Supports chainable configuration for pool parameters  
	pool.InitConfig().             
		WithCleanPeriod(500 * time.Millisecond).
		WithTaskQueueSize(10000).
		WithWorkerNumCapacity(20000)

    // Start the goroutine pool  
	pool.Init()                  

	wait := sync.WaitGroup{}

	for i := 0; i < 20000000; i++ {
		wait.Add(1)

		go func() {
			pool.Submit(func() {
				defer wait.Done()
				time.Sleep(10 * time.Millisecond)
			})
		}()

	}

	wait.Wait()
```

**Pool.SubmitBefore()**    
```go
	pool.SubmitBefore(func() {
		defer wait.Done()
		time.Sleep(10 * time.Millisecond)
	}, 5*time.Second)

```
**benchmark**    
```
BenchmarkAgilePool-14    	       1	5953874200 ns/op	527994424 B/op	21709143 allocs/op
```