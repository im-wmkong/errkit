package http_test

import (
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRenderWithStatus(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(10001, "user_not_found", errkit.DefaultMessage("用户不存在"))
	err := httpext.Status(http.StatusNotFound)(
		K.New(errkit.With("uid", 42)), // attrs 故意不应被渲染
	)

	rec := httptest.NewRecorder()
	httpext.Render(rec, err)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type: %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"code":10001`,
		`"name":"user_not_found"`,
		`"message":"用户不存在"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in body: %s", want, body)
		}
	}
	for _, leak := range []string{`uid`, `attrs`, `cause`} {
		if strings.Contains(body, leak) {
			t.Fatalf("body should not leak %q: %s", leak, body)
		}
	}
}

func TestRenderFallback500(t *testing.T) {
	rec := httptest.NewRecorder()
	httpext.Render(rec, stderrors.New("plain"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("plain error should fall back to 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"message":"plain"`) {
		t.Fatalf("plain error message should still appear, got %s", rec.Body.String())
	}
}

func TestBodyOfNil(t *testing.T) {
	b := httpext.BodyOf(nil)
	if b.Code != 0 || b.Name != "" || b.Message != "" {
		t.Fatalf("nil should yield zero Body, got %+v", b)
	}
}
