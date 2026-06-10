# errkind

[![Go Reference](https://pkg.go.dev/badge/github.com/im-wmkong/errkind.svg)](https://pkg.go.dev/github.com/im-wmkong/errkind)
[![CI](https://github.com/im-wmkong/errkind/actions/workflows/ci.yml/badge.svg)](https://github.com/im-wmkong/errkind/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> 一个面向 Go 1.24+ 的**业务错误建模库** —— 核心是错误的 *领域模型* (Kind 身份 / 实例分离, 零依赖)。<br>
> 协议适配 (HTTP / gRPC / OTel) 与日志库整合 (zap / zerolog / logrus) 由独立的 `ext/` 装饰器与 `integration/*` 子模块提供, 各自按需引入。

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
go get github.com/im-wmkong/errkind
```

最低 Go 版本: **1.24**。

## Quick Start

```go
package main

import (
    stderrors "errors"
    "fmt"
    "log/slog"
    "os"

    "github.com/im-wmkong/errkind"
    httpext "github.com/im-wmkong/errkind/ext/http"
    slogext "github.com/im-wmkong/errkind/ext/slog"
)

// 1. Identity: 一次性 Define, 全局单例。
var UserNotFound = errkind.Define(
    10001,
    "user_not_found",
    errkind.DefaultMessage("用户不存在"),
)

func getUser(id int64) error {
    cause := stderrors.New("sql: no rows in result set")
    // 2. Instance: 每次调用产生新错误。
    err := UserNotFound.Wrap(cause, errkind.With("uid", id))
    // 3. ext 装饰器: 协议字段不污染 core。
    return httpext.Status(404)(err)
}

func main() {
    err := getUser(42)

    // 标准 errors.Is 与 Kind.Is 都能用
    fmt.Println("Is UserNotFound:", UserNotFound.Is(err))

    // 拿到结构化字段
    if c, ok := errkind.CodeOf(err); ok {
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
var UserNotFound = errkind.Define(10001, "user_not_found",
    errkind.DefaultMessage("用户不存在"),
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

c, ok := errkind.CodeOf(err)         // (Code, bool) 推荐
n, ok := errkind.NameOf(err)         // (string, bool)
msg   := errkind.MessageOf(err)      // 不是 errkind 错误时回退到 err.Error()
attrs := errkind.AttrsOf(err)        // 最外层 attrs (拷贝)
flat  := errkind.AllAttrs(err)       // 全链路扁平合并, 外层胜出
```

### Registry (测试隔离 / 多租户)

```go
r := errkind.NewRegistry()
K := r.Define(1, "x")
```

包级 `Define` / `Kinds` / `LookupCode` / `LookupName` 走默认 Registry。

### 调用栈

进程级开关, 默认关闭:

```go
errkind.SetCaptureStack(true)        // 通常在 main 里, dev=true / prod 按需

if t, ok := err.(errkind.Tracer); ok {
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

errkind 把扩展按"是否引入外部依赖"分两层组织:

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
| `integration/grpc` | gRPC `*status.Status` 互转 + 拦截器 (unary 与 streaming) | `ToStatus(err)` / `FromStatus(st)` / `UnaryServerInterceptor()` / `UnaryClientInterceptor()` / `StreamServerInterceptor()` / `StreamClientInterceptor()` |
| `integration/otel` | 把 errkind 字段写到 OTel span | `RecordError(span, err)` / `Attributes(err)` |
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

## 工具链

errkind 提供两个 CLI, 用来在跨团队场景下保证错误码契约一致。

### `cmd/errkindlint` — 错误码冲突静态检查

`Define` 在进程 init 阶段会对重复的 `(code, name)` panic, 但那是运行时。
`errkindlint` 把这层校验前置到编译期: 对整仓 (跨多个 `go.mod`) 的源码做 AST 扫描,
识别 `errkind.Define(...)` 字面量调用并检测冲突。

```bash
go run github.com/im-wmkong/errkind/cmd/errkindlint -exclude=examples/ .
```

- 报告重复 `code` / 重复 `name` / 空 `name`
- 发现冲突退出码非零, 直接接入 CI gate
- `-exclude=glob` 跳过文件 (例如各自独立可运行的 demo, 故意复用错误码)
- 支持 Go 风格的 `./...` 路径

### `cmd/errkind doc` — 错误码文档生成

基于源码静态分析生成稳定的错误码目录, 给前端文案、SRE 告警、客户端 codegen 用。

```bash
go run github.com/im-wmkong/errkind/cmd/errkind doc -format=md  ./...
go run github.com/im-wmkong/errkind/cmd/errkind doc -format=json ./...
go run github.com/im-wmkong/errkind/cmd/errkind doc -format=md -o errors.md ./...
```

Markdown 输出片段:

```
| Code  | Name             | Default Message | Source                |
|------:|------------------|-----------------|-----------------------|
| 10001 | `user_not_found` | 用户不存在        | user/errors.go:12     |
| 10002 | `invalid_argument` | 参数非法        | user/errors.go:18     |
```

两个工具共享 `internal/scan` (纯 AST, 不需要 `init` 真实执行),
即使代码无法编译或带重依赖也能工作。

## 与其他库的对比

| | errkind | `pkg/errors` | `cockroachdb/errors` | 标准库 |
|---|---|---|---|---|
| 业务错误码 | ✅ | ❌ | 半 (字符串 hint) | ❌ |
| Identity / Instance 分离 | ✅ | ❌ | ❌ | ❌ |
| 注册中心 / 冲突检查 | ✅ | ❌ | ❌ | ❌ |
| 兼容 `errors.Is/As` | ✅ | ✅ | ✅ | ✅ |
| 调用栈 | 进程级开关 | 总是抓 | 总是抓 | ❌ |
| HTTP / gRPC 集成 | 装饰器 | ❌ | 内置 | ❌ |
| 核心包零依赖 | ✅ | ✅ | ❌ (拉一堆) | ✅ |

**什么时候选 errkind**: 业务错误码需要被前端 / 客户端 / OTel 维度切分, 且希望领域模型清晰、扩展开放。

**什么时候不选**: 只是想给 error 加 stack 或 fmt 信息——用标准库 `fmt.Errorf("%w", err)` 即可。

## 稳定性

**当前为 v0.x, API 可能变更。** v1.0 将在生产环境验证 ≥6 个月后发布。

## 性能

Apple M-series, Go 1.24, `go test -bench=. -benchtime=2s`:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `New()` 无 Option | 17 | 96 | 1 |
| `New(Message)` | 17 | 96 | 1 |
| `New(With×3)` | 75 | 320 | 4 |
| `Wrap(cause, With)` | 31 | 128 | 2 |
| `New()` 抓栈开 | 198 | 144 | 2 |
| `Kind.Is(err)` | 1.2 | 0 | 0 |
| `CodeOf(err)` | 41 | 8 | 1 |
| `AllAttrs(深度3)` | 91 | 224 | 3 |
| `fmt.Sprintf("%v", err)` | 66 | 80 | 3 |
| `fmt.Sprintf("%+v", err)` 无栈 | 80 | 160 | 4 |
| `json.Marshal(err)` | 717 | 448 | 15 |

跑你自己的环境: `go test -bench=. -benchmem ./...`

## 开发

仓库是多 module 工程 (主 module + 每个 `integration/*` 各自 `go.mod`)。
`go test ./...` **不会跨 module 边界**, 用脚本一键复刻 CI 行为:

```bash
./scripts/test.sh             # 全部 module 跑 vet / build / test
./scripts/test.sh -race       # 透传给 go test
./scripts/test.sh --group     # 发射 GitHub Actions ::group:: 标记 (CI 用)
```

CI test job 直接调这同一个脚本, 单一事实源 —— 本地绿则 CI 这一步绿。

## 已知行为说明

- `Wrap(nil, ...)` 返回 `nil`, 与 `fmt.Errorf("%w", nil)` 一致。
- `errors.Is(err, kind)` **不支持** (`*Kind` 不实现 `error`); 请用 `kind.Is(err)` 或 `errkind.CodeOf(err)`。
- `Define` 重复立即 panic; `(code, name)` 任一冲突或 name 为空都不允许。
- `AttrsOf` 返回拷贝, 修改不影响原错误; `AllAttrs` 同理。

## License

MIT, 详见 [LICENSE](LICENSE)。
