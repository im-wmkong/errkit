package zerolog_test

import (
	"bytes"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	httpext "github.com/im-wmkong/errkit/ext/http"
	zerologext "github.com/im-wmkong/errkit/ext/zerolog"
	"github.com/rs/zerolog"
)

func TestErrFull(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "x", errkit.DefaultMessage("默认"))
	err := K.Wrap(stderrors.New("root"), errkit.With("uid", 9))
	err = httpext.Status(404)(err)
	err = grpcext.Code(5)(err)

	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	logger.Error().Func(zerologext.Err(err)).Msg("fail")

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
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	logger.Error().Func(zerologext.Err(nil)).Msg("fail")
	if !strings.Contains(buf.String(), `"err":{}`) {
		t.Fatalf("expected empty err dict, got:\n%s", buf.String())
	}
}
