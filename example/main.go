package main

import (
	agilepool "agilePool"
	"os"
	"runtime/pprof"
	"sync"
	"time"
)

func main() {
	f, _ := os.Create("cpu_profile.prof")
	pprof.StartCPUProfile(f)     // 开始记录 CPU 使用
	defer pprof.StopCPUProfile() // 程序退出前停止

	// memFile, _ := os.Create("mem_profile.prof")
	// defer func() {
	// 	runtime.GC() // 触发GC减少干扰
	// 	pprof.WriteHeapProfile(memFile)
	// 	memFile.Close()
	// }()

	// go func()
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	// f, _ := os.Create("trace.out")
	// trace.Start(f)
	// defer trace.Stop()

	pool := agilepool.NewPool()

	pool.InitConfig().
		WithBlockMode(agilepool.BLOCK).
		WithCleanPeriod(500 * time.Millisecond).
		WithTaskQueueSize(10000).
		WithWorkerNumCapacity(20000)

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
	pool.SubmitBefore(func() {
		defer wait.Done()
		time.Sleep(10 * time.Millisecond)
	}, 5*time.Second)
	wait.Wait()
}
