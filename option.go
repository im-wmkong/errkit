package errkit

import "fmt"

// Option 作用于 errkit 错误实例自身 (message / attrs)。
//
// 协议相关扩展 (HTTP / gRPC / ...) 不通过 Option, 而是由 ext 包独立装饰器实现,
// 这两个机制不混用——避免一个泛槽位变成什么都往里塞的字典。
type Option func(*kerr)

// Message 设置消息; 多次调用以最后一次为准。
func Message(msg string) Option {
	return func(e *kerr) { e.message = msg }
}

// Messagef 等价于 Message(fmt.Sprintf(format, args...))。
func Messagef(format string, args ...any) Option {
	return func(e *kerr) { e.message = fmt.Sprintf(format, args...) }
}

// With 追加一条 attr; 同名 key 覆盖, 保持原插入位置。
func With(key string, val any) Option {
	return func(e *kerr) { setAttr(&e.attrs, key, val) }
}

// setAttr 是 With 的核心写入逻辑; 抽出来便于将来 WithAttrs 之类批量 Option 复用。
func setAttr(attrs *[]Attr, key string, val any) {
	for i := range *attrs {
		if (*attrs)[i].Key == key {
			(*attrs)[i].Val = val
			return
		}
	}
	*attrs = append(*attrs, Attr{Key: key, Val: val})
}
