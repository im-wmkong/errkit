// Package zap 把 errkit 错误结构化输出到 go.uber.org/zap。
//
//	logger.Error("request failed", zapext.Err(err))
//
// 输出 namespace 字段:
//
//	{"err": {"code":10001,"name":"user_not_found","message":"...",
//	         "attrs":{"uid":42},"http_status":404,"cause":"..."}}
//
// 与 ext/slog 同构, 故意做成显式调用, 避免 core 隐式依赖日志格式。
package zap

import (
	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	httpext "github.com/im-wmkong/errkit/ext/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Err 返回一个 zap.Field, 把 err 的所有结构化字段展开为 namespace。
//
//	logger.Error("...", zapext.Err(err))
func Err(err error) zap.Field {
	return zap.Object("err", marshaler{err: err})
}

// Object 同 Err, 但允许自定义 key:
//
//	logger.Error("...", zapext.Object("biz_err", err))
func Object(key string, err error) zap.Field {
	return zap.Object(key, marshaler{err: err})
}

type marshaler struct{ err error }

func (m marshaler) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	if m.err == nil {
		return nil
	}
	if c, ok := errkit.CodeOf(m.err); ok {
		enc.AddUint32("code", uint32(c))
	}
	if n, ok := errkit.NameOf(m.err); ok {
		enc.AddString("name", n)
	}
	if msg := errkit.MessageOf(m.err); msg != "" {
		enc.AddString("message", msg)
	}
	if all := errkit.AllAttrs(m.err); len(all) > 0 {
		_ = enc.AddObject("attrs", attrsMarshaler(all))
	}
	if c, ok := httpext.StatusOf(m.err); ok {
		enc.AddInt("http_status", c)
	}
	if c, ok := grpcext.CodeOf(m.err); ok {
		enc.AddUint32("grpc_code", uint32(c))
	}
	if cause := unwrapNonErrkit(m.err); cause != nil {
		enc.AddString("cause", cause.Error())
	}
	return nil
}

type attrsMarshaler []errkit.Attr

func (a attrsMarshaler) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	for _, kv := range a {
		if err := enc.AddReflected(kv.Key, kv.Val); err != nil {
			return err
		}
	}
	return nil
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
