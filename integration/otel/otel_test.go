package otel_test

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	httpext "github.com/im-wmkong/errkit/ext/http"
	otelext "github.com/im-wmkong/errkit/ext/otel"
	otelint "github.com/im-wmkong/errkit/integration/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func newSpan(t *testing.T) (*tracetest.SpanRecorder, sdktrace.ReadWriteSpan) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	_, span := tp.Tracer("test").Start(context.Background(), "op")
	return rec, span.(sdktrace.ReadWriteSpan)
}

func attrMap(kvs []attribute.KeyValue) map[string]attribute.Value {
	m := map[string]attribute.Value{}
	for _, kv := range kvs {
		m[string(kv.Key)] = kv.Value
	}
	return m
}

func TestRecordErrorFull(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(10001, "user_not_found", errkit.DefaultMessage("用户不存在"))
	cause := stderrors.New("sql: no rows in result set")
	err := otelext.Name("biz.user.miss")(
		grpcext.Code(5)(
			httpext.Status(404)(
				K.Wrap(cause, errkit.With("uid", 42)),
			),
		),
	)

	rec, span := newSpan(t)
	otelint.RecordError(span, err)
	span.End()

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	s := spans[0]

	if s.Status().Code != otelcodes.Error {
		t.Fatalf("status: %v", s.Status())
	}
	if s.Status().Description != "用户不存在" {
		t.Fatalf("status desc: %q", s.Status().Description)
	}

	got := attrMap(s.Attributes())
	cases := map[string]attribute.Value{
		"err.code":           attribute.Int64Value(10001),
		"err.name":           attribute.StringValue("user_not_found"),
		"err.message":        attribute.StringValue("用户不存在"),
		"err.http_status":    attribute.IntValue(404),
		"err.grpc_code":      attribute.Int64Value(5),
		"err.telemetry_name": attribute.StringValue("biz.user.miss"),
		"err.attrs.uid":      attribute.IntValue(42),
	}
	for k, want := range cases {
		v, ok := got[k]
		if !ok {
			t.Fatalf("missing attr %q in %v", k, got)
		}
		if v.Emit() != want.Emit() {
			t.Fatalf("attr %q: got %v want %v", k, v.Emit(), want.Emit())
		}
	}

	// span 应有一条 RecordError 事件
	if len(s.Events()) == 0 {
		t.Fatal("expected RecordError event")
	}
}

func TestRecordErrorNilSpan(t *testing.T) {
	// 不应 panic
	otelint.RecordError(nil, stderrors.New("x"))
}

func TestRecordErrorNilErr(t *testing.T) {
	rec, span := newSpan(t)
	otelint.RecordError(span, nil)
	span.End()
	if a := rec.Ended()[0].Attributes(); len(a) != 0 {
		t.Fatalf("nil err should write no attrs, got %v", a)
	}
}

func TestAttributesPlainError(t *testing.T) {
	a := otelint.Attributes(stderrors.New("boom"))
	// plain error: 只有 message
	got := attrMap(a)
	if v, ok := got["err.message"]; !ok || v.AsString() != "boom" {
		t.Fatalf("plain error message lost: %v", got)
	}
	for _, k := range []string{"err.code", "err.name", "err.http_status"} {
		if _, ok := got[k]; ok {
			t.Fatalf("plain error should not have %q", k)
		}
	}
}

func TestAttrPrefixOverride(t *testing.T) {
	old := otelint.AttrPrefix
	otelint.AttrPrefix = "biz.err."
	t.Cleanup(func() { otelint.AttrPrefix = old })

	r := errkit.NewRegistry()
	K := r.Define(1, "x")
	a := otelint.Attributes(K.New(errkit.With("foo", "bar")))
	got := attrMap(a)
	if _, ok := got["biz.err.code"]; !ok {
		t.Fatalf("prefix not honored: %v", got)
	}
	if _, ok := got["biz.err.attrs.foo"]; !ok {
		t.Fatalf("attr prefix not honored: %v", got)
	}
}
