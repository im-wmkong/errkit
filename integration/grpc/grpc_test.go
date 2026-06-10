package grpc_test

import (
	"context"
	stderrors "errors"
	"io"
	"net"
	"testing"

	"github.com/im-wmkong/errkind"
	grpcext "github.com/im-wmkong/errkind/ext/grpc"
	grpcint "github.com/im-wmkong/errkind/integration/grpc"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpcsdk "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestToStatusFull(t *testing.T) {
	r := errkind.NewRegistry()
	K := r.Define(10001, "user_not_found", errkind.DefaultMessage("用户不存在"))
	cause := stderrors.New("sql: no rows in result set")
	err := grpcext.Code(uint32(codes.NotFound))(
		K.Wrap(cause, errkind.With("uid", 42)),
	)

	st := grpcint.ToStatus(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("code: %v", st.Code())
	}
	if st.Message() != "用户不存在" {
		t.Fatalf("message: %q", st.Message())
	}
	var info *errdetails.ErrorInfo
	for _, d := range st.Details() {
		if i, ok := d.(*errdetails.ErrorInfo); ok {
			info = i
		}
	}
	if info == nil {
		t.Fatal("ErrorInfo missing")
	}
	if info.Reason != "user_not_found" {
		t.Fatalf("reason: %q", info.Reason)
	}
	if info.Domain != "errkind" {
		t.Fatalf("domain: %q", info.Domain)
	}
	if info.Metadata["uid"] != "42" {
		t.Fatalf("metadata uid: %q", info.Metadata["uid"])
	}
	if info.Metadata["_errkind.code"] != "10001" {
		t.Fatalf("business code missing: %v", info.Metadata)
	}
}

func TestToStatusNil(t *testing.T) {
	if grpcint.ToStatus(nil) != nil {
		t.Fatal("nil should map to nil status")
	}
}

func TestRoundTripPreservesFields(t *testing.T) {
	r := errkind.NewRegistry()
	K := r.Define(10001, "user_not_found", errkind.DefaultMessage("用户不存在"))
	src := grpcext.Code(uint32(codes.NotFound))(
		K.New(errkind.With("uid", 42)),
	)

	st := grpcint.ToStatus(src)
	round := grpcint.FromStatus(st)
	if round == nil {
		t.Fatal("FromStatus returned nil")
	}

	if !grpcint.IsReason(round, "user_not_found") {
		t.Fatal("IsReason failed")
	}
	if c, ok := grpcint.CodeOf(round); !ok || c != 10001 {
		t.Fatalf("CodeOf: %v %v", c, ok)
	}
	if n, ok := grpcint.NameOf(round); !ok || n != "user_not_found" {
		t.Fatalf("NameOf: %v %v", n, ok)
	}
	if g, ok := grpcext.CodeOf(round); !ok || g != uint32(codes.NotFound) {
		t.Fatalf("grpcext.CodeOf: %v %v", g, ok)
	}
	attrs := grpcint.AttrsOf(round)
	found := false
	for _, kv := range attrs {
		if kv.Key == "uid" && kv.Val == "42" {
			found = true
		}
	}
	if !found {
		t.Fatalf("uid attr not roundtripped: %v", attrs)
	}
}

func TestFromStatusOK(t *testing.T) {
	if got := grpcint.FromStatus(status.New(codes.OK, "")); got != nil {
		t.Fatalf("OK should map to nil, got %v", got)
	}
	if got := grpcint.FromStatus(nil); got != nil {
		t.Fatalf("nil should map to nil, got %v", got)
	}
}

func TestFromStatusWithoutErrorInfo(t *testing.T) {
	st := status.New(codes.Internal, "boom")
	err := grpcint.FromStatus(st)
	if err == nil {
		t.Fatal("expected non-nil")
	}
	if g, ok := grpcext.CodeOf(err); !ok || g != uint32(codes.Internal) {
		t.Fatalf("grpc code lost: %v %v", g, ok)
	}
	if _, ok := grpcint.NameOf(err); ok {
		t.Fatal("no ErrorInfo => no name")
	}
}

func TestIsReasonOnLocalErr(t *testing.T) {
	r := errkind.NewRegistry()
	K := r.Define(1, "x")
	if !grpcint.IsReason(K.New(), "x") {
		t.Fatal("local errkind kind should match IsReason")
	}
}

func TestCodeOfFallbackToLocal(t *testing.T) {
	r := errkind.NewRegistry()
	K := r.Define(1, "x")
	if c, ok := grpcint.CodeOf(K.New()); !ok || c != 1 {
		t.Fatalf("local CodeOf fallback failed: %v %v", c, ok)
	}
}

// TestAttrOrderPreservedAcrossRoundTrip 验证 attrs 经 ToStatus -> FromStatus 后,
// 顺序仍是服务端的插入序 (而非 map 随机序)。
func TestAttrOrderPreservedAcrossRoundTrip(t *testing.T) {
	r := errkind.NewRegistry()
	K := r.Define(2001, "ord", errkind.DefaultMessage("o"))
	src := grpcext.Code(uint32(codes.NotFound))(
		K.New(
			errkind.With("z", 1),
			errkind.With("a", 2),
			errkind.With("m", 3),
		),
	)
	st := grpcint.ToStatus(src)
	round := grpcint.FromStatus(st)
	attrs := grpcint.AttrsOf(round)

	wantOrder := []string{"z", "a", "m"}
	if len(attrs) != len(wantOrder) {
		t.Fatalf("len attrs = %d, want %d (%+v)", len(attrs), len(wantOrder), attrs)
	}
	for i, want := range wantOrder {
		if attrs[i].Key != want {
			t.Errorf("attrs[%d].Key = %q, want %q (full: %+v)", i, attrs[i].Key, want, attrs)
		}
	}
}

// TestFromStatusFallbackOrder 验证服务端没编码 _errkind.order 时, 客户端按 key 字典序兜底。
func TestFromStatusFallbackOrder(t *testing.T) {
	st := status.New(codes.Internal, "boom")
	info := &errdetails.ErrorInfo{
		Reason: "x",
		Domain: "errkind",
		Metadata: map[string]string{
			"b": "1",
			"a": "2",
			"c": "3",
		},
	}
	st, derr := st.WithDetails(info)
	if derr != nil {
		t.Fatal(derr)
	}
	err := grpcint.FromStatus(st)
	attrs := grpcint.AttrsOf(err)
	want := []string{"a", "b", "c"}
	if len(attrs) != len(want) {
		t.Fatalf("attrs = %+v", attrs)
	}
	for i, w := range want {
		if attrs[i].Key != w {
			t.Errorf("attrs[%d].Key = %q, want %q", i, attrs[i].Key, w)
		}
	}
}

// TestRemoteErrGRPCStatus 验证 status.FromError 能从 remoteErr 取回原 status。
func TestRemoteErrGRPCStatus(t *testing.T) {
	r := errkind.NewRegistry()
	K := r.Define(3001, "s")
	st := grpcint.ToStatus(grpcext.Code(uint32(codes.NotFound))(K.New()))
	err := grpcint.FromStatus(st)
	got, ok := status.FromError(err)
	if !ok || got.Code() != codes.NotFound {
		t.Fatalf("status.FromError ok=%v code=%v", ok, got.Code())
	}
}

// TestUnaryServerInterceptor_PassThroughGRPCStatus 验证已经是 *status.Status 的错误
// 不会被服务端拦截器二次包装。
func TestUnaryServerInterceptor_PassThroughGRPCStatus(t *testing.T) {
	itc := grpcint.UnaryServerInterceptor()
	want := status.Error(codes.AlreadyExists, "raw")
	handler := func(ctx context.Context, req any) (any, error) { return nil, want }
	_, err := itc(context.Background(), nil, &grpcsdk.UnaryServerInfo{}, handler)
	if err != want {
		t.Fatalf("interceptor wrapped status err: got %v want %v", err, want)
	}
}

// ===== streaming 端到端 =====

// 经 bufconn 起一个真服务, 用流式拦截器跑一次 server-streaming, 验证 Recv 能拿到 errkind 还原错误。
func TestStreamInterceptors_EndToEnd(t *testing.T) {
	r := errkind.NewRegistry()
	K := r.Define(4001, "stream_failed", errkind.DefaultMessage("stream boom"))

	lis := bufconn.Listen(1 << 16)
	defer lis.Close()

	// 用 grpc 的 unknown service handler 注册一个 server-streaming 方法。
	desc := grpcsdk.ServiceDesc{
		ServiceName: "errkind.test.StreamSvc",
		HandlerType: (*any)(nil),
		Streams: []grpcsdk.StreamDesc{{
			StreamName: "Run",
			Handler: func(srv any, ss grpcsdk.ServerStream) error {
				// 服务端故意先发一条, 再返回 errkind 错误。
				_ = ss.SendMsg(&dummyMsg{})
				return grpcext.Code(uint32(codes.FailedPrecondition))(
					K.New(errkind.With("retryable", false)),
				)
			},
			ServerStreams: true,
		}},
	}
	srv := grpcsdk.NewServer(grpcsdk.StreamInterceptor(grpcint.StreamServerInterceptor()))
	srv.RegisterService(&desc, struct{}{})
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	cc, err := grpcsdk.NewClient(
		"passthrough:///bufnet",
		grpcsdk.WithContextDialer(dialer),
		grpcsdk.WithTransportCredentials(insecure.NewCredentials()),
		grpcsdk.WithStreamInterceptor(grpcint.StreamClientInterceptor()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()

	stream, err := cc.NewStream(context.Background(), &desc.Streams[0], "/errkind.test.StreamSvc/Run")
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.SendMsg(&dummyMsg{}); err != nil {
		t.Fatalf("SendMsg: %v", err)
	}
	_ = stream.CloseSend()

	// 第一条正常, 第二次应该拿到错误。
	var msg dummyMsg
	if err := stream.RecvMsg(&msg); err != nil && !stderrors.Is(err, io.EOF) {
		t.Fatalf("first RecvMsg: %v", err)
	}
	rerr := stream.RecvMsg(&msg)
	if rerr == nil {
		t.Fatal("expected error from second RecvMsg")
	}
	if !grpcint.IsReason(rerr, "stream_failed") {
		t.Errorf("IsReason failed: %v", rerr)
	}
	if g, ok := grpcext.CodeOf(rerr); !ok || g != uint32(codes.FailedPrecondition) {
		t.Errorf("grpc code lost: %v %v", g, ok)
	}
	if c, ok := grpcint.CodeOf(rerr); !ok || c != 4001 {
		t.Errorf("business code lost: %v %v", c, ok)
	}
}

// dummyMsg 是一个最简的 proto 消息替身, 用 codec=proto 时会被序列化为空 bytes,
// 满足拦截器测试对消息形态的最低要求。
type dummyMsg struct{}

func (*dummyMsg) Reset()         {}
func (*dummyMsg) String() string { return "" }
func (*dummyMsg) ProtoMessage()  {}
