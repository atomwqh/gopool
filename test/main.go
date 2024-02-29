package main

import (
	"fmt"
	"gopool"
	"sync"
	"sync/atomic"
	"time"
)

var sum int32

func myFunc(i interface{}) {
	n := i.(int32)
	atomic.AddInt32(&sum, n)
	fmt.Printf("run with %d\n", n)
}

func demoFunc() {
	time.Sleep(10 * time.Millisecond)
	fmt.Println("Hello World!")
}

func main() {
	defer gopool.Release()

	runTimes := 1000000

	var wg sync.WaitGroup
	syncCalculateSum := func() {
		demoFunc()
		wg.Done()
	}
	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		_ = gopool.Submit(syncCalculateSum)
	}
	wg.Wait()
	fmt.Printf("running goroutines: %d\n", gopool.Running())
	fmt.Printf("finish all tasks.\n")

	// 带函数的 Pool

}
