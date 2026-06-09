# errkit

[![Go Reference](https://pkg.go.dev/badge/github.com/im-wmkong/errkit.svg)](https://pkg.go.dev/github.com/im-wmkong/errkit)
[![CI](https://github.com/im-wmkong/errkit/actions/workflows/ci.yml/badge.svg)](https://github.com/im-wmkong/errkit/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> 一个面向 Go 1.23+ 的**业务错误建模库**——不是错误处理库, 不是 stack 库, 不是 grpc 库, 而是业务错误领域模型。

简体中文 | [English](README.md)

## 设计原则

**Identity (Kind) 与 Instance (Error) 分离。**

```text
Kind                  Error
 ├─ Code               ├─ Kind  (引用同一身份)
 └─ Name               ├─ Message
                       ├─ Attrs
                       └─ Cause
```

- **Kind** 是错误的"身份"——`(code, name)` 全局单例, 进程启动时一次性 `Define`, 永远不变。
- **Error** 是一次具体错误的实例——每次 `New / Wrap` 产生新对象, 携带 message / attrs / cause。

这样领域模型清晰, 接口极小, 完全兼容 Go 标准错误生态 (`errors.Is` / `errors.As` / `errors.Unwrap`)。

## 安装

```bash
go get github.com/im-wmkong/errkit
```

最低 Go 版本: **1.23**。

## Quick Start

```go
package main

import (
    stderrors "errors"
    "fmt"
    "log/slog"
    "os"

    "github.com/im-wmkong/errkit"
    httpext "github.com/im-wmkong/errkit/ext/http"
    slogext "github.com/im-wmkong/errkit/ext/slog"
)

// 1. Identity: 一次性 Define, 全局单例。
var UserNotFound = errkit.Define(
    10001,
    "user_not_found",
    errkit.DefaultMessage("用户不存在"),
)

func getUser(id int64) error {
    cause := stderrors.New("sql: no rows in result set")
    // 2. Instance: 每次调用产生新错误。
    err := UserNotFound.Wrap(cause, errkit.With("uid", id))
    // 3. ext 装饰器: 协议字段不污染 core。
    return httpext.Status(404)(err)
}

func main() {
    err := getUser(42)

    // 标准 errors.Is 与 Kind.Is 都能用
    fmt.Println("Is UserNotFound:", UserNotFound.Is(err))

    // 拿到结构化字段
    if c, ok := errkit.CodeOf(err); ok {
        fmt.Println("Code:", c)
    }
    if c, ok := httpext.StatusOf(err); ok {
        fmt.Println("HTTP:", c)
    }

    // slog 自动结构化输出
    slog.New(slog.NewJSONHandler(os.Stdout, nil)).
        Error("request failed", slogext.Err(err))
}
```

输出:

```json
{"level":"ERROR","msg":"request failed","err":{
    "code":10001,"name":"user_not_found","message":"用户不存在",
    "attrs":{"uid":42},"http_status":404,
    "cause":"sql: no rows in result set"}}
```

## 核心 API

### 定义 Kind

```go
var UserNotFound = errkit.Define(10001, "user_not_found",
    errkit.DefaultMessage("用户不存在"),
)
```

`Define` 在重复 code/name 时立即 panic, 强制单例。

### 创建 Error

```go
UserNotFound.New(opts...)            // 不带 cause
UserNotFound.Wrap(cause, opts...)    // 包装 cause; cause==nil 返回 nil
```

可用 Option:

| Option | 作用 |
|---|---|
| `Message(s)` | 覆盖默认消息 |
| `Messagef(fmt, args...)` | 格式化消息 |
| `With(k, v)` | 追加 attr (同名覆盖, 顺序保持) |

### 判断与提取

```go
UserNotFound.Is(err)                // true / false
errors.Is(err, sql.ErrNoRows)       // 标准库穿透 cause

c, ok := errkit.CodeOf(err)         // (Code, bool) 推荐
n, ok := errkit.NameOf(err)         // (string, bool)
msg   := errkit.MessageOf(err)      // 不是 errkit 错误时回退到 err.Error()
attrs := errkit.AttrsOf(err)        // 最外层 attrs (拷贝)
flat  := errkit.AllAttrs(err)       // 全链路扁平合并, 外层胜出
```

### Registry (测试隔离 / 多租户)

```go
r := errkit.NewRegistry()
K := r.Define(1, "x")
```

包级 `Define` / `Kinds` / `LookupCode` / `LookupName` 走默认 Registry。

### 调用栈

进程级开关, 默认关闭:

```go
errkit.SetCaptureStack(true)        // 通常在 main 里, dev=true / prod 按需

if t, ok := err.(errkit.Tracer); ok {
    for _, f := range t.StackTrace() { ... }
}
```

不用 `WithStack()` Option 的原因: 「忘记加」是大概率事件, 用进程级开关一刀切。

### 格式化与序列化

```go
fmt.Sprintf("%v",  err)   // 短格式: user_not_found(10001): 用户不存在: <cause>
fmt.Sprintf("%+v", err)   // 多行: 错误信息 + (若抓了栈) 调用栈
fmt.Sprintf("%q",  err)   // 短格式带引号

json.Marshal(err)
// {"code":10001,"name":"user_not_found","message":"用户不存在",
//  "attrs":{"uid":42},"cause":"sql: no rows in result set"}
```

JSON 输出**仅包含 core 字段**, 不含 HTTP/gRPC 等协议字段——后者由 ext 层处理 (例如 `slogext.Err`)。
attrs 按插入顺序输出; 不可序列化的 attr 值 (如 `chan`) 会自动降级为字符串而非整条失败。

## 扩展子包

errkit 把扩展按"是否引入外部依赖"分两层组织:

- **`ext/`** — 协议装饰器, **零外部依赖**, 跟主 module 一起发布。
  用来给错误链挂状态码 / telemetry 命名。
- **`integration/`** — 三方框架重量集成, **每个独立 Go module**,
  避免主 module 被无关依赖污染。

### ext/* (零依赖装饰器, 主 module 内)

| 包 | 用途 | API |
|---|---|---|
| `ext/http` | HTTP 状态码 + JSON 渲染 | `Status(404)(err)` / `StatusOf(err)` / `Render(w, err)` |
| `ext/grpc` | gRPC 状态码 (不引 grpc) | `Code(5)(err)` / `CodeOf(err)` |
| `ext/otel` | Telemetry 命名 (不引 OTel) | `Name("biz.x")(err)` / `NameOf(err)` |
| `ext/slog` | log/slog 集成 (仅标准库) | `Err(err)` / `Value(err)` |

### integration/* (各自独立 module, 各自拉重依赖)

| Module | 用途 | API |
|---|---|---|
| `integration/grpc` | gRPC `*status.Status` 互转 + 拦截器 | `ToStatus(err)` / `FromStatus(st)` / `UnaryServerInterceptor()` / `UnaryClientInterceptor()` |
| `integration/otel` | 把 errkit 字段写到 OTel span | `RecordError(span, err)` / `Attributes(err)` |
| `integration/zap` | go.uber.org/zap | `Err(err)` / `Object(key, err)` |
| `integration/zerolog` | rs/zerolog | `Err(err)` / `Field(key, err)` / `Dict(err)` |
| `integration/logrus` | sirupsen/logrus | `Fields(err)` / `FieldsWithPrefix(prefix, err)` |

日志库示例:

```go
// zap
logger.Error("request failed", zapext.Err(err))

// zerolog
logger.Error().Func(zerologext.Err(err)).Msg("request failed")

// logrus
logger.WithFields(logrusext.Fields(err)).Error("request failed")
```

## 与其他库的对比

| | errkit | `pkg/errors` | `cockroachdb/errors` | 标准库 |
|---|---|---|---|---|
| 业务错误码 | ✅ | ❌ | 半 (字符串 hint) | ❌ |
| Identity / Instance 分离 | ✅ | ❌ | ❌ | ❌ |
| 注册中心 / 冲突检查 | ✅ | ❌ | ❌ | ❌ |
| 兼容 `errors.Is/As` | ✅ | ✅ | ✅ | ✅ |
| 调用栈 | 进程级开关 | 总是抓 | 总是抓 | ❌ |
| HTTP / gRPC 集成 | 装饰器 | ❌ | 内置 | ❌ |
| 核心包零依赖 | ✅ | ✅ | ❌ (拉一堆) | ✅ |

**什么时候选 errkit**: 业务错误码需要被前端 / 客户端 / OTel 维度切分, 且希望领域模型清晰、扩展开放。

**什么时候不选**: 只是想给 error 加 stack 或 fmt 信息——用标准库 `fmt.Errorf("%w", err)` 即可。

## 稳定性

**当前为 v0.x, API 可能变更。** v1.0 将在生产环境验证 ≥6 个月后发布。

## 性能

Apple M-series, Go 1.23, `go test -bench=. -benchtime=2s`:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `New()` 无 Option | 18 | 96 | 1 |
| `New(Message)` | 20 | 96 | 1 |
| `New(With×3)` | 89 | 320 | 4 |
| `Wrap(cause, With)` | 37 | 128 | 2 |
| `New()` 抓栈开 | 190 | 144 | 2 |
| `Kind.Is(err)` | 1.3 | 0 | 0 |
| `CodeOf(err)` | 43 | 8 | 1 |
| `AllAttrs(深度3)` | 96 | 224 | 3 |
| `fmt.Sprintf("%v", err)` | 70 | 80 | 3 |
| `fmt.Sprintf("%+v", err)` 无栈 | 87 | 160 | 4 |
| `json.Marshal(err)` | 817 | 448 | 15 |

跑你自己的环境: `go test -bench=. -benchmem ./...`

## 已知行为说明

- `Wrap(nil, ...)` 返回 `nil`, 与 `fmt.Errorf("%w", nil)` 一致。
- `errors.Is(err, kind)` **不支持** (`*Kind` 不实现 `error`); 请用 `kind.Is(err)` 或 `errkit.CodeOf(err)`。
- `Define` 重复立即 panic; `(code, name)` 任一冲突或 name 为空都不允许。
- `AttrsOf` 返回拷贝, 修改不影响原错误; `AllAttrs` 同理。

## License

MIT, 详见 [LICENSE](LICENSE)。
