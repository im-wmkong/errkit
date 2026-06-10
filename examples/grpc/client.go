package main

import (
	"context"
	"fmt"
	"net"

	grpcext "github.com/im-wmkong/errkind/ext/grpc"
	grpcint "github.com/im-wmkong/errkind/integration/grpc"
	grpcsdk "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// dialClient 用给定的 dialer 建立一条挂了 errkind 客户端拦截器的连接,
// 返回 *grpc.ClientConn, 调用方负责 Close。
func dialClient(dialer func(context.Context, string) (net.Conn, error)) (*grpcsdk.ClientConn, error) {
	return grpcsdk.NewClient(
		"passthrough:///bufnet",
		grpcsdk.WithContextDialer(dialer),
		grpcsdk.WithTransportCredentials(insecure.NewCredentials()),
		grpcsdk.WithUnaryInterceptor(grpcint.UnaryClientInterceptor()),
	)
}

// callGet 发起一次 RPC 并把"客户端拿到错误后的标准分流"演示出来。
func callGet(ctx context.Context, cc *grpcsdk.ClientConn, id int64) {
	fmt.Printf("\n[id=%d]\n", id)
	req := wrapperspb.Int64(id)
	var resp wrapperspb.StringValue
	err := cc.Invoke(ctx, methodName, req, &resp)
	describeClientErr(err)
}

// describeClientErr 演示客户端拿到错误后, 如何用 grpcint / grpcext 取字段并按 reason 分支。
func describeClientErr(err error) {
	if err == nil {
		fmt.Println("  ok")
		return
	}
	fmt.Printf("  error    = %v\n", err)
	if name, ok := grpcint.NameOf(err); ok {
		fmt.Printf("  reason   = %s\n", name)
	}
	if c, ok := grpcint.CodeOf(err); ok {
		fmt.Printf("  bizcode  = %d\n", c)
	}
	if g, ok := grpcext.CodeOf(err); ok {
		fmt.Printf("  grpcCode = %s\n", codes.Code(g))
	}
	for _, kv := range grpcint.AttrsOf(err) {
		fmt.Printf("  attr     = %s=%v\n", kv.Key, kv.Val)
	}
	switch {
	case grpcint.IsReason(err, "user_not_found"):
		fmt.Println("  branch   -> 提示用户去注册")
	case grpcint.IsReason(err, "invalid_argument"):
		fmt.Println("  branch   -> 校验失败, 表单回退")
	}
}
