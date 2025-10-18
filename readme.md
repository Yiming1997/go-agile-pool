## agilePool简介
goAgilePool轻量级的golang goroutine池，简单易用，性能优秀。
## 特性
1. 自定义goroutine池大小
2. 自定义任务队列大小
3. 任务超时控制
4. 自动清理超时的idle workers
5. 使用FIFO对worker队列进行高效地worker复用
## 使用方法
**Pool.Submit()**
```go
    pool := agilepool.NewPool()

	pool.InitConfig().             //协程池参数配置支持链式调用
		WithBlockMode(agilepool.BLOCK).
		WithCleanPeriod(500 * time.Millisecond).
		WithTaskQueueSize(10000).
		WithWorkerNumCapacity(20000)

	pool.Init()                   //启动协程池

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

**Pool.Submit()**    
```go
	pool.SubmitBefore(func() {
		defer wait.Done()
		time.Sleep(10 * time.Millisecond)
	}, 5*time.Second)


```