// Package grpc 把 errkit 错误与 gRPC *status.Status 互转, 并提供拦截器。
//
// 与 ext/grpc 的关系:
//   - ext/grpc 是"轻量层", 只提供 Code(c)(err) / CodeOf(err), 不 import google.golang.org/grpc,
//     适合纯 HTTP 项目零成本携带 gRPC 状态码字段。
//   - integration/grpc 是"重量层", 真正接 google.golang.org/grpc + errdetails,
//     提供 ToStatus / FromStatus / 拦截器, 给真实 gRPC 服务用。
//
// 单独 module: 主 errkit module 不会被 grpc 重依赖污染。
//
//	import grpcint "github.com/im-wmkong/errkit/integration/grpc"
//
//	srv := grpcsdk.NewServer(
//	    grpcsdk.UnaryInterceptor(grpcint.UnaryServerInterceptor()),
//	)
package grpc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpcsdk "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Domain 是 ErrorInfo.Domain 默认值; 业务可在调用前覆盖。
var Domain = "errkit"

// metaCodeKey 是 ErrorInfo.Metadata 中携带 errkit business code 的特殊 key。
//
// 设计取舍: errdetails 标准 Detail 类型里没有"业务码"语义,
// 借 Metadata 走一个保留前缀; "_errkit." 这个前缀业务自定义 attr 通常不会撞。
const metaCodeKey = "_errkit.code"

// ToStatus 把 errkit 错误映射为 *status.Status:
//   - code     <- ext/grpc 装饰; 没有则 codes.Unknown
//   - message  <- errkit.MessageOf
//   - details  <- ErrorInfo{
//     Reason: errkit Kind name,
//     Domain: Domain,
//     Metadata: AllAttrs + {_errkit.code: <business code>},
//     }
//
// nil 入参返回 nil。
func ToStatus(err error) *status.Status {
	if err == nil {
		return nil
	}
	c := codes.Unknown
	if g, ok := grpcext.CodeOf(err); ok {
		c = codes.Code(g)
	}
	st := status.New(c, errkit.MessageOf(err))

	info := &errdetails.ErrorInfo{Domain: Domain, Metadata: map[string]string{}}
	if n, ok := errkit.NameOf(err); ok {
		info.Reason = n
	}
	if bc, ok := errkit.CodeOf(err); ok {
		info.Metadata[metaCodeKey] = strconv.FormatUint(uint64(bc), 10)
	}
	for _, kv := range errkit.AllAttrs(err) {
		info.Metadata[kv.Key] = fmt.Sprint(kv.Val)
	}
	if d, derr := st.WithDetails(info); derr == nil {
		return d
	}
	return st
}

// FromStatus 把对端返回的 *status.Status 还原成一个最薄的 errkit-friendly 错误。
//
// 设计取舍: 不试图反查 Registry (这要求双方共享 Kind 注册中心, 不现实),
// 而是产生一个携带原始 grpc code / business code / name / attrs 的轻量包装,
// 让调用方仍能用本包 CodeOf / NameOf / AttrsOf / IsReason 提取字段,
// 也能用 grpcext.CodeOf 拿到 grpc code。
//
// 注意: 还原出来的错误 *不* 等价于服务端原始 *kerr (没有共享 Kind 身份);
// 若要按 Kind 判等, 应该用 client 自己定义的 Kind + 本包 IsReason。
func FromStatus(st *status.Status) error {
	if st == nil || st.Code() == codes.OK {
		return nil
	}
	var (
		reason       string
		businessCode uint32
		hasBusiness  bool
		attrs        []errkit.Attr
	)
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			reason = info.Reason
			for k, v := range info.Metadata {
				if k == metaCodeKey {
					if n, perr := strconv.ParseUint(v, 10, 32); perr == nil {
						businessCode = uint32(n)
						hasBusiness = true
					}
					continue
				}
				attrs = append(attrs, errkit.Attr{Key: k, Val: v})
			}
			break
		}
	}
	return &remoteErr{
		message:      st.Message(),
		grpcCode:     uint32(st.Code()),
		businessCode: businessCode,
		hasBusiness:  hasBusiness,
		name:         reason,
		attrs:        attrs,
	}
}

// IsReason 检查 err 是否来自远端的某个 ErrorInfo.Reason (即 errkit Kind name)。
//
//	if grpcint.IsReason(err, "user_not_found") { ... }
func IsReason(err error, reason string) bool {
	if r, ok := NameOf(err); ok {
		return r == reason
	}
	return false
}

// CodeOf 提取 errkit business Code (与 grpc code 不同)。先看本地 errkit 错误, 再看 RemoteError。
func CodeOf(err error) (errkit.Code, bool) {
	if c, ok := errkit.CodeOf(err); ok {
		return c, true
	}
	if r, ok := asRemote(err); ok {
		if c, has := r.RemoteBusinessCode(); has {
			return errkit.Code(c), true
		}
	}
	return 0, false
}

// NameOf 提取还原后的 Kind name。先看本地 errkit 错误, 再看 RemoteError。
func NameOf(err error) (string, bool) {
	if n, ok := errkit.NameOf(err); ok {
		return n, true
	}
	if r, ok := asRemote(err); ok {
		if n := r.RemoteName(); n != "" {
			return n, true
		}
	}
	return "", false
}

// AttrsOf 提取还原后的 attrs。先看本地 errkit 错误链, 再看 RemoteError。
func AttrsOf(err error) []errkit.Attr {
	if attrs := errkit.AllAttrs(err); len(attrs) > 0 {
		return attrs
	}
	if r, ok := asRemote(err); ok {
		return r.RemoteAttrs()
	}
	return nil
}

// RemoteError 是 FromStatus 还原出的对端错误的契约。
//
// 命名导出的目的是给"实现者"一个完整契约: remoteErr 编译期保证实现完整,
// 调用者 (CodeOf/NameOf/AttrsOf) 一次断言拿全字段, 减少分散的字面接口与拼写漂移。
//
// 业务一般不需要直接实现这个接口 —— 它由 FromStatus 内部产出;
// 但需要"自定义对端错误类型"时, 实现 RemoteError 即可被本包 helper 识别。
type RemoteError interface {
	error
	GRPCCode() uint32
	RemoteName() string
	RemoteAttrs() []errkit.Attr
	RemoteBusinessCode() (uint32, bool)
}

// asRemote 沿错误链查找第一个 RemoteError。
func asRemote(err error) (RemoteError, bool) {
	for cur := err; cur != nil; {
		if r, ok := cur.(RemoteError); ok {
			return r, true
		}
		u, ok := cur.(interface{ Unwrap() error })
		if !ok {
			break
		}
		cur = u.Unwrap()
	}
	return nil, false
}

// remoteErr 是 FromStatus 还原出来的轻量错误, 实现 RemoteError 契约。
//
// 故意不冒充 *kerr (errkit 内部 extract 只认 *kerr 私有类型),
// 而是通过 RemoteError 接口被本包 helper 识别。
// GRPCCode 让 grpcext.CodeOf 直接可用。
type remoteErr struct {
	message      string
	grpcCode     uint32
	businessCode uint32
	hasBusiness  bool
	name         string
	attrs        []errkit.Attr
}

// 编译期断言: remoteErr 必须满足 RemoteError 契约。
var _ RemoteError = (*remoteErr)(nil)

func (e *remoteErr) Error() string                      { return e.message }
func (e *remoteErr) GRPCCode() uint32                   { return e.grpcCode }
func (e *remoteErr) RemoteName() string                 { return e.name }
func (e *remoteErr) RemoteAttrs() []errkit.Attr         { return append([]errkit.Attr(nil), e.attrs...) }
func (e *remoteErr) RemoteBusinessCode() (uint32, bool) { return e.businessCode, e.hasBusiness }

// UnaryServerInterceptor 在服务端把任意 errkit 错误转成 gRPC status, 客户端就能拿到 ErrorInfo。
//
//	srv := grpcsdk.NewServer(
//	    grpcsdk.UnaryInterceptor(grpcint.UnaryServerInterceptor()),
//	)
func UnaryServerInterceptor() grpcsdk.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpcsdk.UnaryServerInfo, handler grpcsdk.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		// 已经是原生 *status.Status 的不再二次包装。
		// status.FromError 对任意 error 都 ok=true, 必须再判 GRPCStatus 接口。
		if _, isStatus := err.(interface{ GRPCStatus() *status.Status }); isStatus {
			return nil, err
		}
		return nil, ToStatus(err).Err()
	}
}

// UnaryClientInterceptor 在客户端把对端 status 还原成 errkit-friendly 错误,
// 让调用方继续用 errkit.CodeOf / grpcext.CodeOf / 本包 IsReason 提取。
//
//	conn, _ := grpcsdk.NewClient(addr,
//	    grpcsdk.WithUnaryInterceptor(grpcint.UnaryClientInterceptor()),
//	)
func UnaryClientInterceptor() grpcsdk.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpcsdk.ClientConn, invoker grpcsdk.UnaryInvoker, opts ...grpcsdk.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err == nil {
			return nil
		}
		if st, ok := status.FromError(err); ok {
			return FromStatus(st)
		}
		return err
	}
}
