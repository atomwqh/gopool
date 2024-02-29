package gopool

import "time"

type Option func(opts *Options)

func loadOptions(options ...Option) *Options {
	opts := new(Options)
	for _, option := range options {
		option(opts)
	}
	return opts
}

type Options struct {
	// ExpiryDuration 过期的一个时间，回收协程每隔 ExpiryDuration 就会回收一个 ExpiryDuration 没使用过的协程
	ExpiryDuration time.Duration

	// PreAlloc 在开始的时候进行内存的预分配
	PreAlloc bool

	// MaxBlockingTasks 协程池的最大容量，0-表示没有上限
	MaxBlockingTasks int

	// 当 Nonblocking 为 true，Pool.Submit 不会被阻塞，同时 MaxBlockingTasks 也会失效
	// Pool.Submit 无法立即执行的时候会返回 ErrPoolOverload
	Nonblocking bool

	// PanicHandler 主要是为每一个 goWorker 用来处理 panic ，如果是 nil，panic 会被一直抛出
	PanicHandler func(interface{})

	// Logger 标准化日志信息，默认使用 log package
	Logger Logger

	// DisablePurge 为真时，goWorker不会被清除
	DisablePurge bool
}

// WithOptions 接收所有配置
func WithOptions(options Options) Option {
	return func(opts *Options) {
		*opts = options
	}
}

func WithExpiryDuration(expiryDuration time.Duration) Option {
	return func(opts *Options) {
		opts.ExpiryDuration = expiryDuration
	}
}

func WithPreAlloc(preAlloc bool) Option {
	return func(opts *Options) {
		opts.PreAlloc = preAlloc
	}
}

func WithMaxBlockingTasks(maxBlockingTasks int) Option {
	return func(opts *Options) {
		opts.MaxBlockingTasks = maxBlockingTasks
	}
}

// WithNonblocking return nil 表示 没有剩余可用的 goWorker
func WithNonblocking(nonblocking bool) Option {
	return func(opts *Options) {
		opts.Nonblocking = nonblocking
	}
}

func WithPanicHandler(panicHandler func(interface{})) Option {
	return func(opts *Options) {
		opts.PanicHandler = panicHandler
	}
}

func WithLogger(logger Logger) Option {
	return func(opts *Options) {
		opts.Logger = logger
	}
}

// WithDisablePurge 表示自动清理协程池是否开启
func WithDisablePurge(disable bool) Option {
	return func(opts *Options) {
		opts.DisablePurge = disable
	}
}
