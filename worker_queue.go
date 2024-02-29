package gopool

import (
	"errors"
	"time"
)

var (
	// errQueueIsFull Worker 队列满的时候返回
	errQueueIsFull = errors.New("the queue is full")

	// errQueueIsReleased 试图插入被释放的 Worker 队列的时候返回
	errQueueIsReleased = errors.New("the queue length is zero")
)

type worker interface {
	run()
	finish()
	lastUsedTime() time.Time
	inputFunc(func())
	inputParam(interface{})
}

type workerQueue interface {
	len() int
	isEmpty() bool
	insert(worker) error
	detach() worker
	refresh(duration time.Duration) []worker // 清理空闲的 Worker 并且返回
	reset()
}

type queueType int

const (
	queueTypeStack queueType = 1 << iota
	queueTypeLoopQueue
)

// newWorkerQueue 新建 goWorker 队列
func newWorkerQueue(qType queueType, size int) workerQueue {
	switch qType {
	case queueTypeStack:
		return newWorkerStack(size)
	case queueTypeLoopQueue:
		return newWorkerLoopQueue(size)
	default:
		return newWorkerStack(size)
	}
}
