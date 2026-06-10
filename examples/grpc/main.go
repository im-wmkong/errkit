// 演示 errkind 在 gRPC 全链路的真实使用方式: bufconn 起 in-process server + client,
// 经过 integration/grpc 的拦截器把 errkind 错误自动编码 / 解码, 客户端用 IsReason / CodeOf
// 做分支判断, 验证业务码 / Kind name / attrs 跨进程透传。
//
//	go run ./examples/grpc
//
// 关键点:
//   - 业务层只产 errkind 错误 + ext/grpc 装饰; 不直接构造 *status.Status。
//   - 服务端注册 grpcint.UnaryServerInterceptor() 即可统一出协议错误。
//   - 客户端注册 grpcint.UnaryClientInterceptor() 后, invoke 返回的 err 已是 errkind-friendly,
//     可用 grpcint.IsReason / grpcint.CodeOf / grpcext.CodeOf 直接提取字段。
//
// 文件分布:
//   - server.go: Kind 定义 / 业务逻辑 / 服务注册 / 启动
//   - client.go: 拨号 / 调用 / 错误分流
//   - main.go:   仅做协调入口
package main

import (
	"context"
	"net"

	"google.golang.org/grpc/test/bufconn"
)

func main() {
	lis := bufconn.Listen(1 << 16)
	defer lis.Close()

	stop := startServer(lis)
	defer stop()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	cc, err := dialClient(dialer)
	if err != nil {
		panic(err)
	}
	defer cc.Close()

	for _, id := range []int64{42, 0, 999} {
		callGet(context.Background(), cc, id)
	}
}
