package gopool

import (
	"runtime/debug"
	"time"
)

// goWorker 实际运行任务，开启一个协程，接收任务并且返回响应
type goWorker struct {
	pool     *Pool       // goWorker所属的协程池
	task     chan func() // goWorker用于接收异步任务的管道
	lastUsed time.Time   // goWorker上一次被使用的时间，在放到队列时更新
}

// run 启动 Goroutine 的函数 用于重复执行一系列函数调用
func (w *goWorker) run() {
	w.pool.addRunning(1)
	go func() {
		defer func() {
			w.pool.addRunning(-1)
			w.pool.workerCache.Put(w)
			if p := recover(); p != nil {
				if ph := w.pool.options.PanicHandler; ph != nil {
					ph(p)
				} else {
					w.pool.options.Logger.Printf("worker exits from panic: %v\n%s\n", p, debug.Stack())
				}
			}
			// 通知等待中的 Goroutine，让它们知道有可用的工作线程
			w.pool.cond.Signal()
		}()

		for f := range w.task {
			if f == nil {
				return
			}
			f()
			if ok := w.pool.revertWorker(w); !ok {
				return
			}
		}
	}()
}

func (w *goWorker) finish() {
	w.task <- nil
}

func (w *goWorker) lastUsedTime() time.Time {
	return w.lastUsed
}

func (w *goWorker) inputFunc(fn func()) {
	w.task <- fn
}

func (w *goWorker) inputParam(interface{}) {
	panic("unreachable")
}
