package gopool

import (
	"errors"
	"log"
	"math"
	"os"
	"runtime"
	"time"
)

const (
	DefaultAntsPoolSize      = math.MaxInt32
	DefaultCleanIntervalTime = time.Second // 清理协程的一个时间间隔
)

const (
	OPENED = iota
	CLOSED
	// 池的开关，0-开，1-关
)

var (
	ErrLackPoolFunc = errors.New("must provide function for pool")

	// ErrInvalidPoolExpiry 意味着设置一个负数的清理协程的时间间隔
	ErrInvalidPoolExpiry = errors.New("invalid expiry for pool")

	ErrPoolClosed = errors.New("this pool has been closed")

	ErrPoolOverload = errors.New("too many goroutines blocked on submit or Nonblocking is set")

	ErrInvalidPreAllocSize = errors.New("can not set up a negative capacity under PreAlloc mode")

	ErrTimeout = errors.New("operation timed out")

	// ErrInvalidPoolIndex 在一个无效的位置唤醒协程池
	ErrInvalidPoolIndex = errors.New("invalid pool index")

	ErrInvalidLoadBalancingStrategy = errors.New("invalid load-balancing strategy")

	// workerChanCap 决定了worker管道是不是一个缓冲管道以达到最好的性能
	workerChanCap = func() int {
		// runtime.GOMAXPROCS(0)用于获取当前程序使用的CPU数量
		// context 立即转换为接收者，达到最高性能
		if runtime.GOMAXPROCS(0) == 1 {
			return 0
		}
		// 由于接收方受到CPU限制，发送方的性能就会降低
		return 1
	}()

	logLmsgprefix = 64
	defaultLogger = Logger(log.New(os.Stderr, "[ants]: ", log.LstdFlags|logLmsgprefix|log.Lmicroseconds))

	// 引用gopool的时候就会初始化一个协程池
	defaultAntsPool, _ = NewPool(DefaultAntsPoolSize)
)

const nowTimeUpdateInterval = 500 * time.Millisecond // 0.5秒

// Logger 定义日志记录的抽象类
type Logger interface {
	// Printf 格式化输出日志，类似于fmt.Printf，但具体的实现交给具体实现接口的函数
	Printf(format string, args ...interface{})
}
