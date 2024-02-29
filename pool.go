package gopool

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

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

// purgeStaleWorkers 定期清理旧的 goWorker，运行在独立的协程
func (p *Pool) purgeStaleWorkers(ctx context.Context) {
	ticker := time.NewTicker(p.options.ExpiryDuration)
	defer func() {
		ticker.Stop()
		atomic.StoreInt32(&p.purgeDone, 1)
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if p.IsClosed() {
			break
		}

		var isDormant bool
		p.lock.Lock()
		staleWorkers := p.workers.refresh(p.options.ExpiryDuration)
		n := p.Running()
		isDormant = n == 0 || n == len(staleWorkers)
		p.lock.Unlock()

		// 提醒过时的 goWorker 停下
		// 这个提醒必须在 p.lock 之外，因为 w.task 任务有可能被阻塞且耗费大量时间，因为有可能许多 goWorker 不在一个CPU工作
		for i := range staleWorkers {
			staleWorkers[i].finish()
			staleWorkers[i] = nil
		}

		// 这里有一个情况就是所有的 goWorker 都被清理了，但还是有调用被阻塞在 p.cond.Wait()需要被唤醒
		if isDormant && p.Waiting() > 0 {
			p.cond.Broadcast()
		}
	}
}

// ticktock 在协程池中更新时间的一个协程
func (p *Pool) ticktock(ctx context.Context) {
	ticker := time.NewTicker(nowTimeUpdateInterval)
	defer func() {
		ticker.Stop()
		atomic.StoreInt32(&p.ticktockDone, 1)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if p.IsClosed() {
			break
		}

		p.now.Store(time.Now())
	}
}

func (p *Pool) goPurge() {
	if p.options.DisablePurge {
		return
	}

	// 开启一个定时清理过期协程的协程
	var ctx context.Context
	ctx, p.stopPurge = context.WithCancel(context.Background())
	go p.purgeStaleWorkers(ctx)
}

func (p *Pool) goTicktock() {
	p.now.Store(time.Now())
	var ctx context.Context
	ctx, p.stopTicktock = context.WithCancel(context.Background())
	go p.ticktock(ctx)
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

	p.cond = sync.NewCond(p.lock)

	p.goPurge()
	p.goTicktock()

	return p, nil
}

// IsClosed 表示协程池是否关闭
func (p *Pool) IsClosed() bool {
	return atomic.LoadInt32(&p.state) == CLOSED
}

// Running 表示当前正在运行的goWorker数量
func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

// Waiting 返回正在等待执行的任务数量
func (p *Pool) Waiting() int {
	return int(atomic.LoadInt32(&p.waiting))
}
