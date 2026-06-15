package main

import (
	"os"
	"runtime/pprof"
	"sync"
	"time"

	agilepool "github.com/Yiming1997/agilePool"
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

	pool := agilepool.NewPool(agilepool.NewConfig(
		agilepool.WithCleanPeriod(500*time.Millisecond),
		agilepool.WithTaskQueueSize(10000),
		agilepool.WithWorkerNumCapacity(20000),
		agilepool.WithIdleContainerType(agilepool.LinkedListType),
	))

	var submitWG sync.WaitGroup
	for i := 0; i < 20000000; i++ {
		submitWG.Add(1)
		go func() {
			defer submitWG.Done()
			pool.Submit(agilepool.TaskFunc(func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
			}))
		}()
	}

	submitWG.Wait()
	pool.Wait()
}
