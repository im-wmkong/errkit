// Package errkit 是面向 Go 1.22+ 的业务错误建模库。
//
// 设计原则: Identity (Kind) 与 Instance (Error) 分离。
//
//	var UserNotFound = errkit.Define(10001, "user_not_found")
//
//	return UserNotFound.Wrap(cause,
//	    errkit.Message("用户不存在"),
//	    errkit.With("uid", uid),
//	)
//
// core 不感知 HTTP / gRPC / OTel / slog 等任何外部协议;
// 这些扩展位于 ext/* 子包, 通过装饰器 (Decorator) 组合到错误链上,
// 由 errors.As 自然发现, 不依赖任何 core 内部"槽位"。
//
// 文件分布:
//   - errkit.go    包文档 + 共享小类型 (Code / Attr)
//   - kind.go      Kind 身份对象
//   - error.go     kerr 实例, 含 Format / MarshalJSON
//   - option.go    Option 与内置 Message / With
//   - registry.go  Registry + KindOption + 包级默认 Registry
//   - extract.go   从 error 链中提取信息的 helper
//   - stack.go     调用栈 (进程级开关)
package errkit

// Code 是业务错误码的类型。
//
// 故意不用 int, 避免与 HTTP / gRPC 状态码、负数语义混淆;
// uint32 与 grpc/codes.Code 兼容, 与 4 字节序列化也对齐。
type Code uint32

// Attr 是有序键值对; 使用切片而非 map, 保证遍历顺序稳定。
type Attr struct {
	Key string
	Val any
}
