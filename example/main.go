package main

import (
	"os"
	"runtime/pprof"
	"time"

	agilepool "github.com/Yiming1997/go-agile-pool"
)

func main() {
	f, _ := os.Create("cpu_profile.prof")
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	// memFile, _ := os.Create("mem_profile.prof")
	// defer func() {
	// 	runtime.GC() //
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
		WithCleanPeriod(500 * time.Millisecond).
		WithTaskQueueSize(10000).
		WithWorkerNumCapacity(20000)

	pool.Init()

	for i := 0; i < 20000000; i++ {
		go func() {
			pool.Submit(agilepool.TaskFunc(func() {
				time.Sleep(10 * time.Millisecond)

			}))

		}()
	}

	pool.Wait()
}
