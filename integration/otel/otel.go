// Package otel 把 errkit 错误的所有结构化字段写到 OpenTelemetry span 上。
//
//	import otelint "github.com/im-wmkong/errkit/integration/otel"
//
//	if err != nil {
//	    otelint.RecordError(span, err)
//	    return err
//	}
//
// RecordError 等价于:
//
//	span.RecordError(err)                  // 标准事件
//	span.SetStatus(codes.Error, message)   // 标记 span 失败
//	span.SetAttributes(
//	    err.code, err.name, err.message,
//	    err.http_status, err.grpc_code,
//	    err.telemetry_name,
//	    err.attrs.<k>...,                  // errkit AllAttrs 扁平展开
//	)
//
// 与 ext/otel 的关系:
//   - ext/otel 是"轻量层", 只提供 Name(...) 装饰器, 不 import OTel,
//     适合所有项目零成本携带 telemetry name。
//   - integration/otel 是"重量层", 真正接 go.opentelemetry.io/otel,
//     提供 RecordError 一行接入。单独 module, 主 errkit 不被污染。
package otel

import (
	"fmt"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	httpext "github.com/im-wmkong/errkit/ext/http"
	otelext "github.com/im-wmkong/errkit/ext/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// AttrPrefix 是 errkit 业务字段写到 span 时的 namespace 前缀,
// 业务可在调用前覆盖 (例如改为 "biz.err.")。
var AttrPrefix = "err."

// RecordError 把 err 的所有结构化字段写到 span:
//   - span.RecordError(err)
//   - span.SetStatus(codes.Error, errkit.MessageOf)
//   - span.SetAttributes(err.* )
//
// 不抛 panic; nil span 或 nil err 直接返回。
func RecordError(span trace.Span, err error) {
	if span == nil || err == nil || !span.IsRecording() {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, errkit.MessageOf(err))
	if attrs := Attributes(err); len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
}

// Attributes 返回 err 对应的 otel attribute 列表, 让调用方自行决定写入时机:
//
//	span.SetAttributes(otelint.Attributes(err)...)
func Attributes(err error) []attribute.KeyValue {
	if err == nil {
		return nil
	}
	out := make([]attribute.KeyValue, 0, 8)
	if c, ok := errkit.CodeOf(err); ok {
		out = append(out, attribute.Int64(AttrPrefix+"code", int64(c)))
	}
	if n, ok := errkit.NameOf(err); ok {
		out = append(out, attribute.String(AttrPrefix+"name", n))
	}
	if msg := errkit.MessageOf(err); msg != "" {
		out = append(out, attribute.String(AttrPrefix+"message", msg))
	}
	if c, ok := httpext.StatusOf(err); ok {
		out = append(out, attribute.Int(AttrPrefix+"http_status", c))
	}
	if c, ok := grpcext.CodeOf(err); ok {
		out = append(out, attribute.Int64(AttrPrefix+"grpc_code", int64(c)))
	}
	if t := otelext.NameOf(err); t != "" {
		// telemetry_name 与 errkit name 可能不同 (业务显式覆盖); 单独输出便于按维度切分。
		out = append(out, attribute.String(AttrPrefix+"telemetry_name", t))
	}
	for _, kv := range errkit.AllAttrs(err) {
		out = append(out, kvAttr(AttrPrefix+"attrs."+kv.Key, kv.Val))
	}
	return out
}

// kvAttr 把任意 attr 值落到 otel attribute, 不做反射昂贵路径; 不可识别类型回落字符串。
func kvAttr(key string, v any) attribute.KeyValue {
	switch x := v.(type) {
	case string:
		return attribute.String(key, x)
	case bool:
		return attribute.Bool(key, x)
	case int:
		return attribute.Int(key, x)
	case int32:
		return attribute.Int(key, int(x))
	case int64:
		return attribute.Int64(key, x)
	case uint:
		return attribute.Int64(key, int64(x))
	case uint32:
		return attribute.Int64(key, int64(x))
	case uint64:
		return attribute.Int64(key, int64(x))
	case float32:
		return attribute.Float64(key, float64(x))
	case float64:
		return attribute.Float64(key, x)
	default:
		return attribute.String(key, fmt.Sprint(x))
	}
}
