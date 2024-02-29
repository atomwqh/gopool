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

	workerCache sync.Pool

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

func (p *Pool) nowTime() time.Time {
	return p.now.Load().(time.Time)
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

// Submit 提交任务到协程池
func (p *Pool) Submit(task func()) error {
	if p.IsClosed() {
		return ErrPoolClosed
	}

	w, err := p.retrieveWorker()
	if w != nil {
		w.inputFunc(task)
	}
	return err
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

// addRunning 增加一个运行协程
func (p *Pool) addRunning(delta int) {
	atomic.AddInt32(&p.running, int32(delta))
}

// addWaiting 增加一个等待协程
func (p *Pool) addWaiting(delta int) {
	atomic.AddInt32(&p.waiting, int32(delta))
}

// Cap 返回这个 Pool 的容量
func (p *Pool) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

// retrieveWorker 返回一个可用的 goWorker 来运行任务
func (p *Pool) retrieveWorker() (w worker, err error) {
	p.lock.Lock()

retry:
	// 先看看能不能从队列中取出可用的 goWorker
	if w = p.workers.detach(); w != nil {
		p.lock.Unlock()
		return
	}

	// 如果 worker queue 为空，并且还没有用尽 pool 就直接生成一个 goWorker
	if capacity := p.Cap(); capacity == -1 || capacity > p.Running() {
		p.lock.Unlock()
		w = p.workerCache.Get().(*goWorker)
		w.run()
		return
	}
	// 处于 nonblocking 状态或者等待的请求太多了就报错
	if p.options.Nonblocking || (p.options.MaxBlockingTasks != 0 && p.Waiting() >= p.options.MaxBlockingTasks) {
		p.lock.Unlock()
		return nil, ErrPoolOverload
	}

	// 否则只能阻塞住，等待一个可用的 goWorker 放进 Pool
	p.addWaiting(1)
	p.cond.Wait() // 等待一个可用的 goWorker
	p.addWaiting(-1)

	if p.IsClosed() {
		p.lock.Unlock()
		return nil, ErrPoolClosed
	}

	goto retry
}

// revertWorker 将一个 goWorker 放回 Pool ，循环利用池里的 goroutines
func (p *Pool) revertWorker(worker *goWorker) bool {
	if capacity := p.Cap(); (capacity > 0 && p.Running() > capacity) || p.IsClosed() {
		p.cond.Broadcast()
		return false
	}
	worker.lastUsed = p.nowTime()
	p.lock.Lock()
	if p.IsClosed() {
		p.lock.Unlock()
		return false
	}
	if err := p.workers.insert(worker); err != nil {
		p.lock.Unlock()
		return false
	}
	p.cond.Signal()
	p.lock.Unlock()

	return true
}

// Release 关闭这个 Pool 并且释放 worker queue
func (p *Pool) Release() {
	if !atomic.CompareAndSwapInt32(&p.state, OPENED, CLOSED) {
		return
	}

	if p.stopPurge != nil {
		p.stopPurge()
		p.stopPurge = nil
	}
	p.stopTicktock()
	p.stopTicktock = nil

	p.lock.Lock()
	p.workers.reset()
	p.lock.Unlock()

	// 此时可能还有请求等在 retrieveWorker ，所以需要唤醒它们避免永久阻塞
	p.cond.Broadcast()
}
