package main

import (
	"context"
	stderrors "errors"
	"net"

	"github.com/im-wmkong/errkind"
	grpcext "github.com/im-wmkong/errkind/ext/grpc"
	grpcint "github.com/im-wmkong/errkind/integration/grpc"
	grpcsdk "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Kind 是错误的"身份": 在真实项目里通常集中放在 errors 包, 跨 server / client 复用。
// 这里就近放在 server 侧, 强调"业务定义错误码"是服务端的职责。
var (
	UserNotFound = errkind.Define(10001, "user_not_found",
		errkind.DefaultMessage("用户不存在"),
	)
	InvalidArgument = errkind.Define(10002, "invalid_argument",
		errkind.DefaultMessage("参数非法"),
	)
)

var errNoRows = stderrors.New("sql: no rows in result set")

// methodName 在真实项目里来自 protoc 生成代码; 这里手写常量, 避免引入 .proto。
const methodName = "/errkind.example.UserSvc/Get"

// getUser 是模拟的业务逻辑, 只产 errkind 错误。
func getUser(id int64) error {
	switch {
	case id <= 0:
		return grpcext.Code(uint32(codes.InvalidArgument))(
			InvalidArgument.New(errkind.With("id", id)),
		)
	case id == 999:
		return grpcext.Code(uint32(codes.NotFound))(
			UserNotFound.Wrap(errNoRows, errkind.With("uid", id)),
		)
	default:
		return nil
	}
}

// registerService 注册一个最简 unary 方法描述符。
//
// 必须把 dec 解码、业务调用都放进 inner handler, 再交给 interceptor 链;
// 不调 interceptor, errkind 的 UnaryServerInterceptor 就不会被触发, 错误不会被编码。
func registerService(srv *grpcsdk.Server) {
	desc := &grpcsdk.ServiceDesc{
		ServiceName: "errkind.example.UserSvc",
		HandlerType: (*any)(nil),
		Methods: []grpcsdk.MethodDesc{{
			MethodName: "Get",
			Handler: func(_ any, ctx context.Context, dec func(any) error, interceptor grpcsdk.UnaryServerInterceptor) (any, error) {
				var req wrapperspb.Int64Value
				if err := dec(&req); err != nil {
					return nil, err
				}
				inner := func(ctx context.Context, raw any) (any, error) {
					r := raw.(*wrapperspb.Int64Value)
					if err := getUser(r.GetValue()); err != nil {
						return nil, err
					}
					return &wrapperspb.StringValue{Value: "ok"}, nil
				}
				if interceptor == nil {
					return inner(ctx, &req)
				}
				return interceptor(ctx, &req,
					&grpcsdk.UnaryServerInfo{Server: srv, FullMethod: methodName},
					inner)
			},
		}},
	}
	srv.RegisterService(desc, struct{}{})
}

// startServer 创建并启动一个挂载了 errkind unary 拦截器的 gRPC server;
// 调用方提供监听器 (示例里是 bufconn), 返回 stop 函数用于优雅关闭。
func startServer(lis net.Listener) (stop func()) {
	srv := grpcsdk.NewServer(
		grpcsdk.UnaryInterceptor(grpcint.UnaryServerInterceptor()),
	)
	registerService(srv)
	go func() { _ = srv.Serve(lis) }()
	return srv.Stop
}
