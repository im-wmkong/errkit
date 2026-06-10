# errkind

[![Go Reference](https://pkg.go.dev/badge/github.com/im-wmkong/errkind.svg)](https://pkg.go.dev/github.com/im-wmkong/errkind)
[![CI](https://github.com/im-wmkong/errkind/actions/workflows/ci.yml/badge.svg)](https://github.com/im-wmkong/errkind/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> A **business error modeling library** for Go 1.23+ — not an error-handling library, not a stack library, not a gRPC library, but a domain model for business errors.

[简体中文](README_CN.md) | English

## Design Principle

**Identity (Kind) is separated from Instance (Error).**

```text
Kind                  Error
 ├─ Code               ├─ Kind  (refers to the same identity)
 └─ Name               ├─ Message
                       ├─ Attrs
                       └─ Cause
```

- **Kind** is the *identity* of an error — a `(code, name)` global singleton, declared once at process start with `Define`, immutable forever.
- **Error** is a concrete *instance* — every `New / Wrap` produces a fresh object carrying message / attrs / cause.

The result: a clean domain model, a tiny API surface, and full compatibility with the standard error ecosystem (`errors.Is` / `errors.As` / `errors.Unwrap`).

## Install

```bash
go get github.com/im-wmkong/errkind
```

Minimum Go version: **1.23**.

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

// 1. Identity: defined once, global singleton.
var UserNotFound = errkind.Define(
    10001,
    "user_not_found",
    errkind.DefaultMessage("user not found"),
)

func getUser(id int64) error {
    cause := stderrors.New("sql: no rows in result set")
    // 2. Instance: each call yields a new error.
    err := UserNotFound.Wrap(cause, errkind.With("uid", id))
    // 3. ext decorator: protocol fields don't pollute core.
    return httpext.Status(404)(err)
}

func main() {
    err := getUser(42)

    // Both standard errors.Is and Kind.Is work.
    fmt.Println("Is UserNotFound:", UserNotFound.Is(err))

    // Pull out structured fields.
    if c, ok := errkind.CodeOf(err); ok {
        fmt.Println("Code:", c)
    }
    if c, ok := httpext.StatusOf(err); ok {
        fmt.Println("HTTP:", c)
    }

    // slog gets full structured output.
    slog.New(slog.NewJSONHandler(os.Stdout, nil)).
        Error("request failed", slogext.Err(err))
}
```

Output:

```json
{"level":"ERROR","msg":"request failed","err":{
    "code":10001,"name":"user_not_found","message":"user not found",
    "attrs":{"uid":42},"http_status":404,
    "cause":"sql: no rows in result set"}}
```

## Core API

### Define a Kind

```go
var UserNotFound = errkind.Define(10001, "user_not_found",
    errkind.DefaultMessage("user not found"),
)
```

`Define` panics on duplicate code/name to enforce singletons.

### Create an Error

```go
UserNotFound.New(opts...)            // no cause
UserNotFound.Wrap(cause, opts...)    // wrap a cause; cause==nil returns nil
```

Available options:

| Option | Effect |
|---|---|
| `Message(s)` | Override the default message |
| `Messagef(fmt, args...)` | Formatted message |
| `With(k, v)` | Append an attr (same key overwrites, order preserved) |

### Match and Extract

```go
UserNotFound.Is(err)                // true / false
errors.Is(err, sql.ErrNoRows)       // standard library sees through cause

c, ok := errkind.CodeOf(err)         // (Code, bool) — preferred
n, ok := errkind.NameOf(err)         // (string, bool)
msg   := errkind.MessageOf(err)      // falls back to err.Error() for non-errkind errors
attrs := errkind.AttrsOf(err)        // outermost attrs (copy)
flat  := errkind.AllAttrs(err)       // flattened across the chain, outer wins
```

### Registry (test isolation / multi-tenant)

```go
r := errkind.NewRegistry()
K := r.Define(1, "x")
```

Package-level `Define` / `Kinds` / `LookupCode` / `LookupName` use the default registry.

### Stack Trace

A process-level switch, off by default:

```go
errkind.SetCaptureStack(true)        // typically in main; enable in dev as needed

if t, ok := err.(errkind.Tracer); ok {
    for _, f := range t.StackTrace() { ... }
}
```

Why no per-call `WithStack()` option: forgetting to add it is the common case; a process-level toggle is the right default.

### Formatting and Serialization

```go
fmt.Sprintf("%v",  err)   // short:  user_not_found(10001): user not found: <cause>
fmt.Sprintf("%+v", err)   // multi-line: error info + (if captured) stack trace
fmt.Sprintf("%q",  err)   // short, quoted

json.Marshal(err)
// {"code":10001,"name":"user_not_found","message":"user not found",
//  "attrs":{"uid":42},"cause":"sql: no rows in result set"}
```

JSON output **only contains core fields**, never protocol fields like HTTP / gRPC — those are handled by the ext layer (e.g. `slogext.Err`).
Attrs are emitted in insertion order; an attr value that can't be serialized (e.g. `chan`) is downgraded to a string instead of failing the whole marshal.

## Extensions

errkind organizes extensions into two layers:

- **`ext/`** — protocol decorators with **zero external dependencies**. Always available
  inside the main module. Use to attach status codes / telemetry names to the error chain.
- **`integration/`** — heavy integrations with third-party frameworks. Each lives in its
  **own Go module**, so the main module stays free of unrelated dependencies.

### ext/* (zero-dep decorators, in main module)

| Package | Purpose | API |
|---|---|---|
| `ext/http` | HTTP status code + JSON renderer | `Status(404)(err)` / `StatusOf(err)` / `Render(w, err)` |
| `ext/grpc` | gRPC status code (no grpc dep) | `Code(5)(err)` / `CodeOf(err)` |
| `ext/otel` | Telemetry name (no OTel dep) | `Name("biz.x")(err)` / `NameOf(err)` |
| `ext/slog` | log/slog integration (stdlib only) | `Err(err)` / `Value(err)` |

### integration/* (separate modules, each pulls its own deps)

| Module | Purpose | API |
|---|---|---|
| `integration/grpc` | gRPC `*status.Status` round-trip + interceptors (unary & streaming) | `ToStatus(err)` / `FromStatus(st)` / `UnaryServerInterceptor()` / `UnaryClientInterceptor()` / `StreamServerInterceptor()` / `StreamClientInterceptor()` |
| `integration/otel` | Write errkind fields onto OTel spans | `RecordError(span, err)` / `Attributes(err)` |
| `integration/zap` | go.uber.org/zap | `Err(err)` / `Object(key, err)` |
| `integration/zerolog` | rs/zerolog | `Err(err)` / `Field(key, err)` / `Dict(err)` |
| `integration/logrus` | sirupsen/logrus | `Fields(err)` / `FieldsWithPrefix(prefix, err)` |

Logger snippets:

```go
// zap
logger.Error("request failed", zapext.Err(err))

// zerolog
logger.Error().Func(zerologext.Err(err)).Msg("request failed")

// logrus
logger.WithFields(logrusext.Fields(err)).Error("request failed")
```

## Tooling

errkind ships two CLIs to keep error codes consistent across teams.

### `cmd/errkindlint` — static collision check

`Define` panics on duplicate `(code, name)` at process init. `errkindlint` brings
that check forward to compile time by statically scanning `errkind.Define(...)`
literal calls across the whole repo (and across multiple `go.mod` boundaries).

```bash
go run github.com/im-wmkong/errkind/cmd/errkindlint -exclude=examples/ .
```

- Reports duplicate `code`, duplicate `name`, and empty `name`
- Exit code is non-zero on findings, ready for CI gating
- `-exclude=glob` skips files (e.g. independent demo programs that intentionally reuse codes)
- Accepts Go-style `./...` paths

### `cmd/errkind doc` — error code documentation

Generates a stable error-code catalogue from source — useful for frontend
copywriters, SRE alert configs, and client codegen.

```bash
go run github.com/im-wmkong/errkind/cmd/errkind doc -format=md  ./...
go run github.com/im-wmkong/errkind/cmd/errkind doc -format=json ./...
go run github.com/im-wmkong/errkind/cmd/errkind doc -format=md -o errors.md ./...
```

Markdown output (excerpt):

```
| Code  | Name             | Default Message | Source                |
|------:|------------------|-----------------|-----------------------|
| 10001 | `user_not_found` | 用户不存在       | user/errors.go:12     |
| 10002 | `invalid_argument` | 参数非法       | user/errors.go:18     |
```

Both tools share `internal/scan` (AST-only, no `init` execution required), so
they work even on code that fails to compile or has heavy framework deps.

## Comparison with Other Libraries

| | errkind | `pkg/errors` | `cockroachdb/errors` | stdlib |
|---|---|---|---|---|
| Business error code | ✅ | ❌ | partial (string hint) | ❌ |
| Identity / Instance separation | ✅ | ❌ | ❌ | ❌ |
| Registry / collision check | ✅ | ❌ | ❌ | ❌ |
| Compatible with `errors.Is/As` | ✅ | ✅ | ✅ | ✅ |
| Stack trace | process toggle | always | always | ❌ |
| HTTP / gRPC integration | decorator | ❌ | built-in | ❌ |
| Zero-dep core package | ✅ | ✅ | ❌ (heavy) | ✅ |

**When to choose errkind**: business error codes need to be sliced by frontend / clients / OTel dimensions, and you want a clean domain model with open extension points.

**When not to**: you only want to attach a stack or a wrapping message to an error — `fmt.Errorf("%w", err)` is enough.

## Stability

**Currently v0.x; the API may still change.** v1.0 will be cut after ≥6 months of production validation.

## Performance

Apple M-series, Go 1.23, `go test -bench=. -benchtime=2s`:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `New()` no option | 18 | 96 | 1 |
| `New(Message)` | 20 | 96 | 1 |
| `New(With×3)` | 89 | 320 | 4 |
| `Wrap(cause, With)` | 37 | 128 | 2 |
| `New()` with stack capture | 190 | 144 | 2 |
| `Kind.Is(err)` | 1.3 | 0 | 0 |
| `CodeOf(err)` | 43 | 8 | 1 |
| `AllAttrs(depth=3)` | 96 | 224 | 3 |
| `fmt.Sprintf("%v", err)` | 70 | 80 | 3 |
| `fmt.Sprintf("%+v", err)` no stack | 87 | 160 | 4 |
| `json.Marshal(err)` | 817 | 448 | 15 |

Run on your own box: `go test -bench=. -benchmem ./...`

## Development

The repo is a multi-module Go workspace (main module + each `integration/*`
under its own `go.mod`). `go test ./...` does **not** cross module boundaries,
so use the bundled script to mirror CI locally:

```bash
./scripts/test.sh             # vet / build / test for every module
./scripts/test.sh -race       # extra args are forwarded to `go test`
./scripts/test.sh --group     # GitHub Actions ::group:: markers (used by CI)
```

The same script is the single source of truth for the CI test job — anything
green locally is green on CI for those steps.

## Known Behavior Notes

- `Wrap(nil, ...)` returns `nil`, matching `fmt.Errorf("%w", nil)`.
- `errors.Is(err, kind)` is **not supported** (`*Kind` does not implement `error`); use `kind.Is(err)` or `errkind.CodeOf(err)`.
- `Define` panics on duplicates; either `code` or `name` colliding, or an empty `name`, is rejected.
- `AttrsOf` returns a copy — mutations don't affect the original error; same goes for `AllAttrs`.

## License

MIT, see [LICENSE](LICENSE).
