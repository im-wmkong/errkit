package grpc_test

import (
	stderrors "errors"
	"testing"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	grpcint "github.com/im-wmkong/errkit/integration/grpc"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToStatusFull(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(10001, "user_not_found", errkit.DefaultMessage("用户不存在"))
	cause := stderrors.New("sql: no rows in result set")
	err := grpcext.Code(uint32(codes.NotFound))(
		K.Wrap(cause, errkit.With("uid", 42)),
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
	if info.Domain != "errkit" {
		t.Fatalf("domain: %q", info.Domain)
	}
	if info.Metadata["uid"] != "42" {
		t.Fatalf("metadata uid: %q", info.Metadata["uid"])
	}
	if info.Metadata["_errkit.code"] != "10001" {
		t.Fatalf("business code missing: %v", info.Metadata)
	}
}

func TestToStatusNil(t *testing.T) {
	if grpcint.ToStatus(nil) != nil {
		t.Fatal("nil should map to nil status")
	}
}

func TestRoundTripPreservesFields(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(10001, "user_not_found", errkit.DefaultMessage("用户不存在"))
	src := grpcext.Code(uint32(codes.NotFound))(
		K.New(errkit.With("uid", 42)),
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
	r := errkit.NewRegistry()
	K := r.Define(1, "x")
	if !grpcint.IsReason(K.New(), "x") {
		t.Fatal("local errkit kind should match IsReason")
	}
}

func TestCodeOfFallbackToLocal(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "x")
	if c, ok := grpcint.CodeOf(K.New()); !ok || c != 1 {
		t.Fatalf("local CodeOf fallback failed: %v %v", c, ok)
	}
}
