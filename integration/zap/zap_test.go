package zap_test

import (
	"bytes"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	httpext "github.com/im-wmkong/errkit/ext/http"
	zapext "github.com/im-wmkong/errkit/integration/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newJSONLogger(buf *bytes.Buffer) *zap.Logger {
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(buf), zapcore.DebugLevel)
	return zap.New(core)
}

func TestErrFull(t *testing.T) {
	r := errkit.NewRegistry()
	K := r.Define(1, "x", errkit.DefaultMessage("默认"))
	err := K.Wrap(stderrors.New("root"), errkit.With("uid", 9))
	err = httpext.Status(404)(err)
	err = grpcext.Code(5)(err)

	var buf bytes.Buffer
	logger := newJSONLogger(&buf)
	logger.Error("fail", zapext.Err(err))
	_ = logger.Sync()

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
	logger := newJSONLogger(&buf)
	logger.Error("fail", zapext.Err(nil))
	_ = logger.Sync()
	if !strings.Contains(buf.String(), `"err":{}`) {
		t.Fatalf("expected empty err object, got:\n%s", buf.String())
	}
}
