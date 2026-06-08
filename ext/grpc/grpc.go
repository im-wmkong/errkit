// Package grpc 把 gRPC 状态码装饰到错误链上。
//
//	import (
//	    grpcext "github.com/im-wmkong/errkit/ext/grpc"
//	    "google.golang.org/grpc/codes"
//	)
//
//	err = grpcext.Code(uint32(codes.NotFound))(err)
//
// 故意不 import grpc/codes, 避免给纯 HTTP 项目拉额外依赖;
// uint32 与 codes.Code 二进制兼容, 互转即可。
package grpc

import "errors"

type withCode struct {
	error
	code uint32
}

func (w *withCode) GRPCCode() uint32 { return w.code }
func (w *withCode) Unwrap() error    { return w.error }

// Code 返回装饰器, 给 error 附加 gRPC 状态码。
func Code(c uint32) func(error) error {
	return func(err error) error {
		if err == nil {
			return nil
		}
		return &withCode{error: err, code: c}
	}
}

// CodeOf 沿错误链查找 gRPC 状态码。
func CodeOf(err error) (uint32, bool) {
	var s interface{ GRPCCode() uint32 }
	if errors.As(err, &s) {
		return s.GRPCCode(), true
	}
	return 0, false
}
