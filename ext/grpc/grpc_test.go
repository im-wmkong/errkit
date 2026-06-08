package grpc_test

import (
	stderrors "errors"
	"testing"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
)

func TestCode(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "x")
	err := grpcext.Code(5)(K.New())
	if c, ok := grpcext.CodeOf(err); !ok || c != 5 {
		t.Fatalf("got %v %v", c, ok)
	}
}

func TestCodeOnNil(t *testing.T) {
	if got := grpcext.Code(5)(nil); got != nil {
		t.Fatalf("nil decorate should stay nil, got %v", got)
	}
}

func TestCodeOfPlain(t *testing.T) {
	if _, ok := grpcext.CodeOf(stderrors.New("plain")); ok {
		t.Fatal("plain error should not have grpc code")
	}
}

// 验证装饰器 Unwrap 让错误链穿透 (errors.Is 找到 cause)。
func TestUnwrapThroughCodeDecorator(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "x")
	cause := stderrors.New("boom")
	err := grpcext.Code(5)(K.Wrap(cause))

	if !stderrors.Is(err, cause) {
		t.Fatal("errors.Is should still find cause through decorator")
	}
	if !K.Is(err) {
		t.Fatal("Kind.Is should still match")
	}
}
