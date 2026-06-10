// Package grpc 把 errkind 错误与 gRPC *status.Status 互转, 并提供拦截器。
//
// 与 ext/grpc 的关系:
//   - ext/grpc 是"轻量层", 只提供 Code(c)(err) / CodeOf(err), 不 import google.golang.org/grpc,
//     适合纯 HTTP 项目零成本携带 gRPC 状态码字段。
//   - integration/grpc 是"重量层", 真正接 google.golang.org/grpc + errdetails,
//     提供 ToStatus / FromStatus / 拦截器, 给真实 gRPC 服务用。
//
// 单独 module: 主 errkind module 不会被 grpc 重依赖污染。
//
//	import grpcint "github.com/im-wmkong/errkind/integration/grpc"
//
//	srv := grpcsdk.NewServer(
//	    grpcsdk.UnaryInterceptor(grpcint.UnaryServerInterceptor()),
//	)
package grpc

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/im-wmkong/errkind"
	grpcext "github.com/im-wmkong/errkind/ext/grpc"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpcsdk "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Domain 是 ErrorInfo.Domain 默认值; 业务可在调用前覆盖。
var Domain = "errkind"

// metaCodeKey 是 ErrorInfo.Metadata 中携带 errkind business code 的特殊 key。
//
// 设计取舍: errdetails 标准 Detail 类型里没有"业务码"语义,
// 借 Metadata 走一个保留前缀; "_errkind." 这个前缀业务自定义 attr 通常不会撞。
const metaCodeKey = "_errkind.code"

// metaOrderKey 把服务端 attrs 的插入顺序编码进 metadata, 客户端按此顺序还原,
// 以保证 attrs 的"按插入序"承诺跨网络也成立; 没有该 key 时客户端兜底按 key 字典序。
const metaOrderKey = "_errkind.order"

// ToStatus 把 errkind 错误映射为 *status.Status:
//   - code     <- ext/grpc 装饰; 没有则 codes.Unknown
//   - message  <- errkind.MessageOf
//   - details  <- ErrorInfo{
//     Reason: errkind Kind name,
//     Domain: Domain,
//     Metadata: AllAttrs + {_errkind.code: <business code>},
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
	st := status.New(c, errkind.MessageOf(err))

	info := &errdetails.ErrorInfo{Domain: Domain, Metadata: map[string]string{}}
	if n, ok := errkind.NameOf(err); ok {
		info.Reason = n
	}
	if bc, ok := errkind.CodeOf(err); ok {
		info.Metadata[metaCodeKey] = strconv.FormatUint(uint64(bc), 10)
	}
	attrs := errkind.AllAttrs(err)
	if len(attrs) > 0 {
		order := make([]string, 0, len(attrs))
		seen := map[string]struct{}{}
		for _, kv := range attrs {
			if _, dup := seen[kv.Key]; dup {
				// AllAttrs 已经做了去重 (外层覆盖内层), 这里防御性兜底; 不破坏外层值。
				continue
			}
			seen[kv.Key] = struct{}{}
			info.Metadata[kv.Key] = fmt.Sprint(kv.Val)
			order = append(order, kv.Key)
		}
		info.Metadata[metaOrderKey] = encodeOrder(order)
	}
	if d, derr := st.WithDetails(info); derr == nil {
		return d
	}
	return st
}

// FromStatus 把对端返回的 *status.Status 还原成一个最薄的 errkind-friendly 错误。
//
// 设计取舍: 不试图反查 Registry (这要求双方共享 Kind 注册中心, 不现实),
// 而是产生一个携带原始 grpc code / business code / name / attrs 的轻量包装,
// 让调用方仍能用本包 CodeOf / NameOf / AttrsOf / IsReason 提取字段,
// 也能用 grpcext.CodeOf 拿到 grpc code。
//
// 注意: 还原出来的错误 *不* 等价于服务端原始 *kerr (没有共享 Kind 身份);
// 若要按 Kind 判等, 应该用 client 自己定义的 Kind + 本包 IsReason。
//
// attrs 顺序: 优先按服务端编码的 _errkind.order 还原; 缺失时按 key 字典序兜底,
// 这样不同次调用的同一错误在客户端遍历顺序稳定。
func FromStatus(st *status.Status) error {
	if st == nil || st.Code() == codes.OK {
		return nil
	}
	var (
		reason       string
		businessCode uint32
		hasBusiness  bool
		attrs        []errkind.Attr
	)
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			reason = info.Reason
			attrs = decodeAttrs(info.Metadata, &businessCode, &hasBusiness)
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
		status:       st,
	}
}

// decodeAttrs 从 metadata 中过滤掉 _errkind.* 保留 key, 按 _errkind.order 编排剩余 attrs;
// 没有 order 时按 key 字典序兜底, 业务可读且跨调用稳定。
func decodeAttrs(meta map[string]string, businessCode *uint32, hasBusiness *bool) []errkind.Attr {
	if len(meta) == 0 {
		return nil
	}
	if v, ok := meta[metaCodeKey]; ok {
		if n, perr := strconv.ParseUint(v, 10, 32); perr == nil {
			*businessCode = uint32(n)
			*hasBusiness = true
		}
	}
	order := decodeOrder(meta[metaOrderKey])
	if len(order) == 0 {
		// 兜底: 按 key 字典序, 但跳过保留 key。
		for k := range meta {
			if isReservedKey(k) {
				continue
			}
			order = append(order, k)
		}
		sort.Strings(order)
	}
	out := make([]errkind.Attr, 0, len(order))
	for _, k := range order {
		if isReservedKey(k) {
			continue
		}
		v, ok := meta[k]
		if !ok {
			continue
		}
		out = append(out, errkind.Attr{Key: k, Val: v})
	}
	return out
}

func isReservedKey(k string) bool { return k == metaCodeKey || k == metaOrderKey }

// IsReason 检查 err 是否来自远端的某个 ErrorInfo.Reason (即 errkind Kind name)。
//
//	if grpcint.IsReason(err, "user_not_found") { ... }
func IsReason(err error, reason string) bool {
	if r, ok := NameOf(err); ok {
		return r == reason
	}
	return false
}

// CodeOf 提取 errkind business Code (与 grpc code 不同)。先看本地 errkind 错误, 再看 RemoteError。
func CodeOf(err error) (errkind.Code, bool) {
	if c, ok := errkind.CodeOf(err); ok {
		return c, true
	}
	if r, ok := asRemote(err); ok {
		if c, has := r.RemoteBusinessCode(); has {
			return errkind.Code(c), true
		}
	}
	return 0, false
}

// NameOf 提取还原后的 Kind name。先看本地 errkind 错误, 再看 RemoteError。
func NameOf(err error) (string, bool) {
	if n, ok := errkind.NameOf(err); ok {
		return n, true
	}
	if r, ok := asRemote(err); ok {
		if n := r.RemoteName(); n != "" {
			return n, true
		}
	}
	return "", false
}

// AttrsOf 提取还原后的 attrs。先看本地 errkind 错误链, 再看 RemoteError。
func AttrsOf(err error) []errkind.Attr {
	if attrs := errkind.AllAttrs(err); len(attrs) > 0 {
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
	RemoteAttrs() []errkind.Attr
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
// 故意不冒充 *kerr (errkind 内部 extract 只认 *kerr 私有类型),
// 而是通过 RemoteError 接口被本包 helper 识别。
// GRPCCode 让 grpcext.CodeOf 直接可用; GRPCStatus 让 status.FromError 还原为同一个 status;
// Unwrap 暂返回 nil (没有可还原的"内部 cause", 否则要再编码一层 errdetails)。
type remoteErr struct {
	message      string
	grpcCode     uint32
	businessCode uint32
	hasBusiness  bool
	name         string
	attrs        []errkind.Attr
	status       *status.Status
}

// 编译期断言: remoteErr 必须满足 RemoteError 契约。
var _ RemoteError = (*remoteErr)(nil)

func (e *remoteErr) Error() string                      { return e.message }
func (e *remoteErr) GRPCCode() uint32                   { return e.grpcCode }
func (e *remoteErr) RemoteName() string                 { return e.name }
func (e *remoteErr) RemoteAttrs() []errkind.Attr        { return append([]errkind.Attr(nil), e.attrs...) }
func (e *remoteErr) RemoteBusinessCode() (uint32, bool) { return e.businessCode, e.hasBusiness }

// GRPCStatus 让 google.golang.org/grpc/status.FromError 仍然能精确还原 *status.Status,
// 业务在客户端拿到 remoteErr 也能继续 status.Convert(err).Details() 自取。
func (e *remoteErr) GRPCStatus() *status.Status { return e.status }

// encodeOrder 把 attrs key 列表用 \x1f (Unit Separator) 拼接;
// 选 \x1f 是因为它不会出现在合法的 attr key 里, 也不会被 protobuf metadata 转义。
func encodeOrder(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	out := keys[0]
	for _, k := range keys[1:] {
		out += "\x1f" + k
	}
	return out
}

// decodeOrder 是 encodeOrder 的逆操作; 空串视作"没有顺序信息"。
func decodeOrder(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1f' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// UnaryServerInterceptor 在服务端把任意 errkind 错误转成 gRPC status, 客户端就能拿到 ErrorInfo。
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

// UnaryClientInterceptor 在客户端把对端 status 还原成 errkind-friendly 错误,
// 让调用方继续用 errkind.CodeOf / grpcext.CodeOf / 本包 IsReason 提取。
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

// StreamServerInterceptor 与 UnaryServerInterceptor 对应, 用于 server-streaming /
// client-streaming / bidi: 把 handler 返回的 errkind 错误统一转成 *status.Status。
//
//	srv := grpcsdk.NewServer(
//	    grpcsdk.StreamInterceptor(grpcint.StreamServerInterceptor()),
//	)
func StreamServerInterceptor() grpcsdk.StreamServerInterceptor {
	return func(srv any, ss grpcsdk.ServerStream, info *grpcsdk.StreamServerInfo, handler grpcsdk.StreamHandler) error {
		err := handler(srv, ss)
		if err == nil {
			return nil
		}
		if _, isStatus := err.(interface{ GRPCStatus() *status.Status }); isStatus {
			return err
		}
		return ToStatus(err).Err()
	}
}

// StreamClientInterceptor 与 UnaryClientInterceptor 对应:
// 在 NewStream 失败 / RecvMsg 失败时把 status 还原成 errkind-friendly 错误。
//
//	conn, _ := grpcsdk.NewClient(addr,
//	    grpcsdk.WithStreamInterceptor(grpcint.StreamClientInterceptor()),
//	)
func StreamClientInterceptor() grpcsdk.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpcsdk.StreamDesc, cc *grpcsdk.ClientConn, method string, streamer grpcsdk.Streamer, opts ...grpcsdk.CallOption) (grpcsdk.ClientStream, error) {
		cs, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			if st, ok := status.FromError(err); ok {
				return nil, FromStatus(st)
			}
			return nil, err
		}
		return &errkindClientStream{ClientStream: cs}, nil
	}
}

// errkindClientStream 包装 grpcsdk.ClientStream, 把 SendMsg / RecvMsg / CloseSend
// 返回的 status 错误透明地映射成 errkind-friendly 错误; io.EOF 等非 status 错误原样返回。
//
// Header() / Trailer() / Context() 不需要重写: 它们不返回业务 status 错误。
type errkindClientStream struct {
	grpcsdk.ClientStream
}

func (s *errkindClientStream) SendMsg(m any) error { return mapStreamErr(s.ClientStream.SendMsg(m)) }
func (s *errkindClientStream) RecvMsg(m any) error { return mapStreamErr(s.ClientStream.RecvMsg(m)) }
func (s *errkindClientStream) CloseSend() error    { return mapStreamErr(s.ClientStream.CloseSend()) }

func mapStreamErr(err error) error {
	if err == nil {
		return nil
	}
	if _, isStatus := err.(interface{ GRPCStatus() *status.Status }); isStatus {
		if st, ok := status.FromError(err); ok {
			return FromStatus(st)
		}
	}
	return err
}
