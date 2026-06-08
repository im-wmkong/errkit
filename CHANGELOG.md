# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 与 [SemVer](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.1.0] - 2026-06-08

首个公开版本。**API 处于 v0.x 阶段, 后续小版本可能存在不兼容变更。**

### Added

#### core

- `Kind` 身份对象 + `Registry` 注册中心
  - `Define(code, name, ...KindOption)` 重复 (code, name) 立即 panic
  - `KindOption`: `DefaultMessage`
  - `Kinds() / LookupCode / LookupName` 用于错误码文档生成
  - `NewRegistry()` 支持测试隔离 / 多租户
- 错误实例 `*kerr` (不导出, 通过 `Kind.New / Kind.Wrap` 构造)
  - `Wrap(nil, ...)` 返回 `nil`, 与 `fmt.Errorf("%w", nil)` 一致
  - 完全兼容 `errors.Is` / `errors.As` / `errors.Unwrap`
- `Option`: `Message`, `Messagef`, `With`
- 提取 helper: `KindOf`, `CodeOf` / `NameOf` (nil-safe), `MessageOf`, `AttrsOf` (拷贝), `AllAttrs` (扁平合并)
- `fmt.Formatter` 实现:
  - `%v / %s` 单行
  - `%+v` 多行 (含调用栈, 若开启)
  - `%q` 加引号
- `json.Marshaler` 实现:
  - 输出 `code / name / message / attrs / cause`
  - attrs 按插入顺序输出
  - 不可序列化的 attr 值降级为字符串, 不会让整条日志失败
- 调用栈:
  - `SetCaptureStack(bool)` 进程级开关 (默认关)
  - `Tracer` 接口 + `[]Frame` 延迟解析
- `Code` 类型化为 `uint32` (与 `grpc/codes.Code` 二进制兼容)

#### ext (装饰器风格, 不依赖 core 内部"槽位")

- `ext/http`: `Status(code int)` / `StatusOf(err) (int, bool)`
- `ext/grpc`: `Code(c uint32)` / `CodeOf(err) (uint32, bool)`
- `ext/otel`: `Name(s string)` / `NameOf(err) string` (兜底到 `Kind.Name()`)
- `ext/slog`:
  - `Err(err) slog.Attr` / `Value(err) slog.Value`
  - 自动展开 `code / name / message / attrs / http_status / grpc_code / cause`

#### 工程

- `examples/basic` 最小可运行示例
- 单元测试 + benchmark (`Kind.Is` 1.3ns 零分配, `New` 18ns 单分配)
- GitHub Actions CI: 多 Go 版本矩阵 (1.22 / 1.23 / 1.24) × (Linux / macOS) + `staticcheck` + `govulncheck`
- README 含定位、quick start、与 `pkg/errors` / `cockroachdb/errors` 对比、性能数字、已知行为说明

### Known limitations

- `errors.Is(err, kind)` **不支持** (`*Kind` 不实现 `error`); 请用 `kind.Is(err)` 或 `errkit.CodeOf(err)`
- 暂未提供错误码冲突的静态检查工具 (规划在 v0.x)
- 暂未提供 i18n / metrics 自动发射 (规划在 v0.x)

[Unreleased]: https://github.com/im-wmkong/errkit/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/im-wmkong/errkit/releases/tag/v0.1.0
