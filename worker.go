package gopool

import "time"

type goWorker struct {
	pool     *Pool       // goWorker所属的协程池
	task     chan func() // goWorker用于接收异步任务的管道
	lastUsed time.Time   // goWorker上一次被使用的时间，在放到队列时更新
}
