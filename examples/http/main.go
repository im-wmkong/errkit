// 演示 errkit 在 net/http 服务里如何统一渲染错误响应。
//
//	go run ./examples/http
//	curl -i http://127.0.0.1:8080/user?id=42        # 200
//	curl -i http://127.0.0.1:8080/user?id=0         # 400
//	curl -i http://127.0.0.1:8080/user?id=999       # 404
//
// 关键点:
//   - 业务层只产 errkit 错误 + ext/http 装饰, 不直接碰 ResponseWriter。
//   - httpext.Render 是协议出口, 决定 status + body 形状, 只暴露 code/name/message。
//   - 服务端日志保留全部信息 (含 cause/attrs), 由 slogext 处理。
package main

import (
	stderrors "errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/im-wmkong/errkit"
	httpext "github.com/im-wmkong/errkit/ext/http"
	slogext "github.com/im-wmkong/errkit/ext/slog"
)

var (
	UserNotFound = errkit.Define(10001, "user_not_found",
		errkit.DefaultMessage("用户不存在"),
	)
	InvalidArgument = errkit.Define(10002, "invalid_argument",
		errkit.DefaultMessage("参数非法"),
	)
	Internal = errkit.Define(10500, "internal",
		errkit.DefaultMessage("内部错误"),
	)
)

var errNoRows = stderrors.New("sql: no rows in result set")

func fakeDB(id int64) error {
	if id == 999 {
		return errNoRows
	}
	return nil
}

func getUser(id int64) error {
	if id <= 0 {
		return httpext.Status(http.StatusBadRequest)(
			InvalidArgument.New(errkit.With("id", id)),
		)
	}
	if err := fakeDB(id); err != nil {
		if stderrors.Is(err, errNoRows) {
			return httpext.Status(http.StatusNotFound)(
				UserNotFound.Wrap(err, errkit.With("uid", id)),
			)
		}
		return httpext.Status(http.StatusInternalServerError)(
			Internal.Wrap(err),
		)
	}
	return nil
}

func handleUser(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err := getUser(id); err != nil {
			logger.Error("request failed", slogext.Err(err)) // 服务端: 全量
			httpext.Render(w, err)                            // 客户端: 安全字段
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"id":` + strconv.FormatInt(id, 10) + `,"name":"alice"}`))
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	mux := http.NewServeMux()
	mux.Handle("/user", handleUser(logger))

	addr := ":8080"
	logger.Info("listening", slog.String("addr", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("server exited", slogext.Err(err))
		os.Exit(1)
	}
}
