// Package logrus 把 errkit 错误结构化输出到 github.com/sirupsen/logrus。
//
//	logger.WithFields(logrusext.Fields(err)).Error("request failed")
//
// 输出 fields:
//
//	{"err.code":10001, "err.name":"user_not_found", "err.message":"...",
//	 "err.attrs.uid":42, "err.http_status":404, "err.cause":"..."}
//
// 与 ext/slog / integration/zap / integration/zerolog 同构。logrus 没有原生 nested object
// 概念, 这里采用扁平 dot-key 风格输出, 与多数 logrus 用户的实践一致;
// 想保持嵌套对象的请用 ext/slog / integration/zap / integration/zerolog。
package logrus

import (
	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	httpext "github.com/im-wmkong/errkit/ext/http"
	"github.com/sirupsen/logrus"
)

// Fields 把 err 转成 logrus.Fields, 默认 prefix 为 "err":
//
//	logger.WithFields(logrusext.Fields(err)).Error("...")
func Fields(err error) logrus.Fields {
	return FieldsWithPrefix("err", err)
}

// FieldsWithPrefix 同 Fields, 但允许自定义 key 前缀:
//
//	logger.WithFields(logrusext.FieldsWithPrefix("biz_err", err)).Error("...")
func FieldsWithPrefix(prefix string, err error) logrus.Fields {
	f := logrus.Fields{}
	if err == nil {
		return f
	}
	p := prefix
	if p != "" {
		p = prefix + "."
	}
	if c, ok := errkit.CodeOf(err); ok {
		f[p+"code"] = uint32(c)
	}
	if n, ok := errkit.NameOf(err); ok {
		f[p+"name"] = n
	}
	if msg := errkit.MessageOf(err); msg != "" {
		f[p+"message"] = msg
	}
	for _, kv := range errkit.AllAttrs(err) {
		f[p+"attrs."+kv.Key] = kv.Val
	}
	if c, ok := httpext.StatusOf(err); ok {
		f[p+"http_status"] = c
	}
	if c, ok := grpcext.CodeOf(err); ok {
		f[p+"grpc_code"] = uint32(c)
	}
	if cause := unwrapNonErrkit(err); cause != nil {
		f[p+"cause"] = cause.Error()
	}
	return f
}

// unwrapNonErrkit 找到错误链上"最底层"的非 nil cause; 用于 cause 字段输出。
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
