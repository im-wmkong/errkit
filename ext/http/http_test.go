package http_test

import (
	stderrors "errors"
	"testing"

	"github.com/im-wmkong/errkit"
	httpext "github.com/im-wmkong/errkit/ext/http"
)

func TestStatusBasic(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "x")
	err := httpext.Status(404)(K.New())

	c, ok := httpext.StatusOf(err)
	if !ok || c != 404 {
		t.Fatalf("got %v %v", c, ok)
	}
}

func TestStatusOnNil(t *testing.T) {
	if got := httpext.Status(404)(nil); got != nil {
		t.Fatalf("nil decorate should stay nil, got %v", got)
	}
}

func TestStatusUnwrapsToErrkit(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "x")
	cause := stderrors.New("boom")
	err := httpext.Status(409)(K.Wrap(cause))

	if !K.Is(err) {
		t.Fatal("Kind.Is should still work through decorator")
	}
	if !stderrors.Is(err, cause) {
		t.Fatal("errors.Is should still find cause")
	}
}

func TestStatusOfPlainError(t *testing.T) {
	if _, ok := httpext.StatusOf(stderrors.New("plain")); ok {
		t.Fatal("plain error has no status")
	}
}
