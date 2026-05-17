## Introduction
goAgilePool is a lightweight goroutine pool for Golang, designed for simplicity and high performance
## Features
1. Customizable goroutine pool size
2. Configurable task queue size
3. Task timeout control
4. Automatic cleanup of idle workers upon timeout
5. Configurable idle worker container (LinkedList / MinHeap)
6. Task with retry times

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
**TaskWithRetry**  
go-agile-pool allows us to submit a task with a retry count. The task will be retried automatically if it encounters an error.
```go
agilePool.Submit(&agilepool.TaskWithRetry{
		MinBackOff: 1 * time.Second,
		MaxBackOff: 200 * time.Second,
		RetryNum:   3,
		Task: func() error {
			times++
			log.Println("getting err over here")
			return errors.New("err")
		},
	})
```

**IdleWorkerContainer**  
go-agile-pool supports pluggable idle worker container implementations. You can choose between `LinkedList` (default, FIFO) and `MinHeap` (ordered by `lastActiveAt`) to manage idle workers, depending on your scenario.

```go
pool := agilepool.NewPool()
pool.InitConfig().
    WithIdleContainerType(agilepool.MinHeapType).
    WithWorkerNumCapacity(20000)
pool.Init()
```

| Container | Ordered By | Pop | RemoveExpired | Use Case |
|-----------|-----------|-----|---------------|----------|
| `LinkedListType` (default) | Insertion time (FIFO) | First added worker | Full traversal O(n) | General purpose, simple FIFO reuse |
| `MinHeapType` | `lastActiveAt` | Least recently active worker | Early termination O(k log n) | Efficient expiration cleanup |

**Benchmark**   
The benchmark suite measures pool throughput under four scenarios, crossing two dimensions:
- **Submit style**: concurrent (`go func`) vs sequential (direct call)
- **Idle container**: `MinHeap` (ordered by `lastActiveAt`) vs `LinkedList` (FIFO)

Each benchmark submits 10 million tasks (each `time.Sleep(10ms)`) and waits for completion.

Run all benchmarks:
```bash
go test -bench=. -benchtime=1x -timeout=2h -run=^$ -count=1
```

Run a single variant:
```bash
go test -bench=BenchmarkAgilePoolMinHeap -benchtime=1x -timeout=2h -run=^$ -count=1
```

| Benchmark | Submit | Idle Container |
|---|---|---|
| `BenchmarkAgilePoolMinHeap` | concurrent (`go func`) | `MinHeapType` |
| `BenchmarkAgilePoolLinkedList` | concurrent (`go func`) | `LinkedListType` |
| `BenchmarkAgilePoolSequentialMinHeap` | sequential | `MinHeapType` |
| `BenchmarkAgilePoolSequentialLinkedList` | sequential | `LinkedListType` |