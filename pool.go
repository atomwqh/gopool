package gopool

import (
	"context"
	"sync"
	"sync/atomic"

	syncx "gopool/sync"
)

type Pool struct {
	capacity int32 // 池子的容量

	running int32 // 运行中的协程数量

	lock sync.Locker // 自制的自旋锁，保证goWorker的并发安全

	workers workerQueue // 真正意义上的协程池

	state int32 // 池子的状态，0-开启；1-关闭，主要用于提醒池子自己关闭

	cond *sync.Cond // 等待一个闲置的goWorker

	workerCache sync.Pool //

	waiting int32 // 标识出处于等待状态的协程数量

	purgeDone int32

	stopPurge context.CancelFunc

	ticktockDone int32
	stopTicktock context.CancelFunc

	now atomic.Value

	options *Options
}

// NewPool 按照配置实例化一个协程池
func NewPool(size int, options ...Option) (*Pool, error) {
	if size <= 0 {
		size = -1
	}
	opts := loadOptions(options...)

	if !opts.DisablePurge {
		if expiry := opts.ExpiryDuration; expiry < 0 {
			return nil, ErrInvalidPoolExpiry
		} else if expiry == 0 {
			opts.ExpiryDuration = DefaultCleanIntervalTime
		}
	}

	if opts.Logger == nil {
		opts.Logger = defaultLogger
	}

	p := &Pool{
		capacity: int32(size),
		lock:     syncx.NewSpinLock(),
		options:  opts,
	}
	// 初始化状态
	p.workerCache.New = func() interface{} {
		return &goWorker{
			pool: p,
			task: make(chan func(), workerChanCap),
		}
	}

	if p.options.PreAlloc {
		if size == -1 {
			return nil, ErrInvalidPreAllocSize
		}
		p.workers = newWorkerQueue(queueTypeLoopQueue, size)
	} else {
		p.workers = newWorkerQueue(queueTypeStack, 0)
	}
}
