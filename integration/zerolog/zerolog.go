// Package zerolog 把 errkit 错误结构化输出到 github.com/rs/zerolog。
//
//	logger.Error().Func(zerologext.Err(err)).Msg("request failed")
//
// 输出 dict 字段:
//
//	{"err": {"code":10001,"name":"user_not_found","message":"...",
//	         "attrs":{"uid":42},"http_status":404,"cause":"..."}}
//
// 与 ext/slog / integration/zap 同构, 故意做成显式调用, 避免 core 隐式依赖日志格式。
package zerolog

import (
	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	httpext "github.com/im-wmkong/errkit/ext/http"
	"github.com/rs/zerolog"
)

// Err 返回一个适用于 zerolog.Event.Func 的函数, 在 "err" key 下注入错误字段。
//
//	logger.Error().Func(zerologext.Err(err)).Msg("...")
func Err(err error) func(*zerolog.Event) {
	return Field("err", err)
}

// Field 同 Err, 但允许自定义 key:
//
//	logger.Error().Func(zerologext.Field("biz_err", err)).Msg("...")
func Field(key string, err error) func(*zerolog.Event) {
	return func(e *zerolog.Event) {
		e.Dict(key, Dict(err))
	}
}

// Dict 把 err 转成 *zerolog.Event (dict 形式), 由调用者自行挂到日志上。
func Dict(err error) *zerolog.Event {
	d := zerolog.Dict()
	if err == nil {
		return d
	}
	if c, ok := errkit.CodeOf(err); ok {
		d = d.Uint32("code", uint32(c))
	}
	if n, ok := errkit.NameOf(err); ok {
		d = d.Str("name", n)
	}
	if msg := errkit.MessageOf(err); msg != "" {
		d = d.Str("message", msg)
	}
	if all := errkit.AllAttrs(err); len(all) > 0 {
		ad := zerolog.Dict()
		for _, kv := range all {
			ad = ad.Interface(kv.Key, kv.Val)
		}
		d = d.Dict("attrs", ad)
	}
	if c, ok := httpext.StatusOf(err); ok {
		d = d.Int("http_status", c)
	}
	if c, ok := grpcext.CodeOf(err); ok {
		d = d.Uint32("grpc_code", uint32(c))
	}
	if cause := unwrapNonErrkit(err); cause != nil {
		d = d.Str("cause", cause.Error())
	}
	return d
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
