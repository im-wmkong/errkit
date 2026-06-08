// Package http 把 HTTP 状态码装饰到错误链上。
//
//	import httpext "github.com/im-wmkong/errkit/ext/http"
//
//	err := UserNotFound.New(errkit.Message("..."))
//	err  = httpext.Status(404)(err)            // 装饰一次
//
//	if c, ok := httpext.StatusOf(err); ok { w.WriteHeader(c) }
//
// 设计要点: 这是一个独立的 wrapper, 不依赖 errkit 内部任何"槽位"。
// errors.As 自然能找到它, errors.Is 自然能穿透它。
package http

import "errors"

// withStatus 是装饰器产生的具体错误; 不导出, 强制走构造函数。
type withStatus struct {
	error
	code int
}

func (w *withStatus) HTTPStatus() int { return w.code }
func (w *withStatus) Unwrap() error    { return w.error }

// Status 返回一个装饰器: 给传入的 error 附加 HTTP 状态码。
//
//	err = httpext.Status(404)(err)
func Status(code int) func(error) error {
	return func(err error) error {
		if err == nil {
			return nil
		}
		return &withStatus{error: err, code: code}
	}
}

// StatusOf 沿错误链查找 HTTP 状态码; 没有返回 (0, false)。
//
// 任何实现了 HTTPStatus() int 的错误都能被识别, 不必经过 ext/http 装饰。
func StatusOf(err error) (int, bool) {
	var s interface{ HTTPStatus() int }
	if errors.As(err, &s) {
		return s.HTTPStatus(), true
	}
	return 0, false
}
