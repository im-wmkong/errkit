// Package slog 把 errkit 错误结构化输出到 log/slog。
//
//	logger.Error("request failed", slogext.Err(err))
//
// 输出 group 字段:
//
//	{"err": {"code":10001,"name":"user_not_found","message":"...",
//	         "attrs":{"uid":42},"http_status":404,"cause":"..."}}
//
// 故意把 slog 集成做成显式调用, 而不是给 *kerr 加 LogValue 方法——
// 那样会让 core 隐式依赖日志格式约定。
package slog

import (
	"log/slog"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	httpext "github.com/im-wmkong/errkit/ext/http"
)

// Err 返回一个 slog.Attr, 把 err 的所有结构化字段展开为 group。
//
//	slog.Error("...", slogext.Err(err))
func Err(err error) slog.Attr {
	return slog.Attr{Key: "err", Value: Value(err)}
}

// Value 把 err 转成 slog.Value (group 形式), 便于自定义 key:
//
//	slog.Any("biz_err", slogext.Value(err))
func Value(err error) slog.Value {
	if err == nil {
		return slog.AnyValue(nil)
	}

	attrs := make([]slog.Attr, 0, 6)
	if c, ok := errkit.CodeOf(err); ok {
		attrs = append(attrs, slog.Uint64("code", uint64(c)))
	}
	if n, ok := errkit.NameOf(err); ok {
		attrs = append(attrs, slog.String("name", n))
	}
	if msg := errkit.MessageOf(err); msg != "" {
		attrs = append(attrs, slog.String("message", msg))
	}
	if all := errkit.AllAttrs(err); len(all) > 0 {
		ma := make([]slog.Attr, 0, len(all))
		for _, a := range all {
			ma = append(ma, slog.Any(a.Key, a.Val))
		}
		attrs = append(attrs, slog.Attr{Key: "attrs", Value: slog.GroupValue(ma...)})
	}
	if c, ok := httpext.StatusOf(err); ok {
		attrs = append(attrs, slog.Int("http_status", c))
	}
	if c, ok := grpcext.CodeOf(err); ok {
		attrs = append(attrs, slog.Uint64("grpc_code", uint64(c)))
	}
	if cause := unwrapNonErrkit(err); cause != nil {
		attrs = append(attrs, slog.String("cause", cause.Error()))
	}
	return slog.GroupValue(attrs...)
}

// unwrapNonErrkit 找到错误链上"最底层"的非 nil cause; 用于 cause 字段输出。
//
// 对于多层 errkit 嵌套, 我们只关心最终原始 cause (例如 sql.ErrNoRows),
// 中间的 errkit Kind 已经被 attrs/name 体现。
func unwrapNonErrkit(err error) error {
	var last error
	for cur := err; cur != nil; {
		if _, ok := errkit.CodeOf(cur); !ok {
			last = cur
		}
		u, ok := cur.(interface{ Unwrap() error })
		if !ok {
			break
		}
		cur = u.Unwrap()
	}
	return last
}
