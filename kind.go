package errkit

// Kind 是错误的"身份"—— code + name 组成的全局单例, 永远不变。
//
// Kind 由 Registry.Define 创建, 不能直接 new; 重复 (code, name) 会 panic。
type Kind struct {
	code           Code
	name           string
	defaultMessage string
}

// Code 返回业务错误码。
func (k *Kind) Code() Code { return k.code }

// Name 返回稳定的、可被日志/OTel/指标使用的标识 (推荐 snake_case)。
func (k *Kind) Name() string { return k.name }

// DefaultMessage 返回默认消息, 可能为空。
func (k *Kind) DefaultMessage() string { return k.defaultMessage }

// New 创建一个不带 cause 的新错误实例。
func (k *Kind) New(opts ...Option) error {
	return k.build(nil, opts)
}

// Wrap 包装一个 cause; cause == nil 时返回 nil, 与 fmt.Errorf("%w", nil) 一致。
func (k *Kind) Wrap(cause error, opts ...Option) error {
	if cause == nil {
		return nil
	}
	return k.build(cause, opts)
}

// Is 判断 err 链上是否含有本 Kind 的实例 (按 Kind 指针相等)。
//
// 不依赖 code / name 字符串比较——避免不同 Registry 同 code 误判。
func (k *Kind) Is(err error) bool {
	for cur := err; cur != nil; {
		if e, ok := cur.(*kerr); ok && e.kind == k {
			return true
		}
		u, ok := cur.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		cur = u.Unwrap()
	}
	return false
}

// build 是 New / Wrap 的共同实现。
func (k *Kind) build(cause error, opts []Option) error {
	e := &kerr{
		kind:    k,
		message: k.defaultMessage,
		cause:   cause,
	}
	for _, o := range opts {
		o(e)
	}
	if captureStack.Load() && !hasStack(cause) {
		e.pcs = capturePCs(3)
	}
	return e
}
