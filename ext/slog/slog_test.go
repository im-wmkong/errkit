package slog_test

import (
	"bytes"
	"context"
	stderrors "errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	httpext "github.com/im-wmkong/errkit/ext/http"
	slogext "github.com/im-wmkong/errkit/ext/slog"
)

func TestErrFull(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "x", errkit.DefaultMessage("默认"))
	err := K.Wrap(stderrors.New("root"), errkit.With("uid", 9))
	err = httpext.Status(404)(err)
	err = grpcext.Code(5)(err)

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.LogAttrs(context.Background(), slog.LevelError, "fail", slogext.Err(err))

	out := buf.String()
	for _, w := range []string{
		`"name":"x"`,
		`"code":1`,
		`"message":"默认"`,
		`"uid":9`,
		`"http_status":404`,
		`"grpc_code":5`,
		`"cause":"root"`,
	} {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in:\n%s", w, out)
		}
	}
}

func TestErrNil(t *testing.T) {
	v := slogext.Value(nil)
	if v.Kind() != slog.KindAny {
		t.Fatalf("nil should produce any-kind, got %v", v.Kind())
	}
}
