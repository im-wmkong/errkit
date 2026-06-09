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

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/im-wmkong/errkit"
)

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

// Body 是 Render 写入响应体的形状; 只暴露给客户端的安全字段,
// 不含 cause / attrs (那是服务端日志的事)。
type Body struct {
	Code    uint32 `json:"code,omitempty"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// BodyOf 把 errkit 错误抽成 Body, 适合自定义渲染时机/形状的场景:
//
//	b := httpext.BodyOf(err)
//	b.Message = i18n.T(b.Name)        // 比如做翻译
//	json.NewEncoder(w).Encode(b)
func BodyOf(err error) Body {
	if err == nil {
		return Body{}
	}
	b := Body{Message: errkit.MessageOf(err)}
	if c, ok := errkit.CodeOf(err); ok {
		b.Code = uint32(c)
	}
	if n, ok := errkit.NameOf(err); ok {
		b.Name = n
	}
	return b
}

// Render 把 err 统一写到 ResponseWriter:
//   - status: ext/http 装饰; 没有则 500
//   - Content-Type: application/json; charset=utf-8
//   - body: BodyOf(err) 的 JSON
//
// 故意不做日志: 调用方决定用 slog/zap/zerolog/logrus 哪一个。
//
//	if err := getUser(id); err != nil {
//	    httpext.Render(w, err)
//	    return
//	}
func Render(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if c, ok := StatusOf(err); ok {
		status = c
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(BodyOf(err))
}
