package otel_test

import (
	stderrors "errors"
	"testing"

	"github.com/im-wmkong/errkit"
	otelext "github.com/im-wmkong/errkit/ext/otel"
)

func TestNameOverride(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "kind_name")
	err := otelext.Name("biz.x")(K.New())
	if got := otelext.NameOf(err); got != "biz.x" {
		t.Fatalf("override wrong: %s", got)
	}
}

func TestNameFallbackToKind(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "kind_name")
	if got := otelext.NameOf(K.New()); got != "kind_name" {
		t.Fatalf("fallback wrong: %s", got)
	}
}

func TestNameOnNil(t *testing.T) {
	if got := otelext.Name("x")(nil); got != nil {
		t.Fatalf("nil decorate should stay nil, got %v", got)
	}
}

func TestNameOfPlainError(t *testing.T) {
	if got := otelext.NameOf(stderrors.New("plain")); got != "" {
		t.Fatalf("plain error should give empty name, got %q", got)
	}
}

func TestUnwrapThroughNameDecorator(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "x")
	cause := stderrors.New("boom")
	err := otelext.Name("biz.x")(K.Wrap(cause))

	if !stderrors.Is(err, cause) {
		t.Fatal("errors.Is should still find cause")
	}
	if !K.Is(err) {
		t.Fatal("Kind.Is should still match")
	}
}
