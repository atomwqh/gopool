package gopool

import "time"

type Option func(opts *Options)

func loadOptions(options ...Options) *Options {
	opts := new(Options)
	for _, option := range options {
		option(opts)
	}
	return opts
}

type Options struct {
	//
	ExpiryDuration time.Duration
	//
	PreAlloc bool
	//
	MaxBlockingTasks int
	Nonblocking      bool
	PanicHandler     func(interface{})
	Logger           Logger
	DisablePurge     bool
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

func WithDisablePurge(disable bool) Option {
	return func(opts *Options) {
		opts.DisablePurge = disable
	}
}
