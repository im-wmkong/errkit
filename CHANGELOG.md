# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 与 [SemVer](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.1.3] - 2026-06-09

### Changed

- **BREAKING**: 项目从 `errkit` 重命名为 `errkind`
  - 模块路径由 `github.com/im-wmkong/errkit` 调整为 `github.com/im-wmkong/errkind`
  - 文件 `errkit.go` / `errkit_test.go` / `errkit_bench_test.go` 重命名为 `errkind.{go,_test.go,_bench_test.go}`
  - README / 示例 / 集成包内的导入路径与标识符同步更新

### Fixed

- `integration/grpc`: 修正业务码 (business code) 注释前缀, 避免 godoc 渲染异常

## [0.1.2] - 2026-06-09

### Added

#### integration (端到端集成包, 各自独立 go.mod)

- `integration/grpc`: gRPC 服务端拦截器与状态码映射, 把 `*Kind` / `*kerr` 转成 `status.Status`, 携带业务码与 attrs
- `integration/otel`: OpenTelemetry 集成, 在 span 上记录错误码、名称、attrs, 支持 `RecordError`
- `integration/logrus`: Logrus Hook / Field 适配, 自动展开错误结构化字段
- `integration/zap`: Zap Field 适配, 零反射展开 attrs
- `integration/zerolog`: Zerolog Event 适配, 输出 code / name / attrs / cause

#### examples

- `examples/http`: 基于标准库 `net/http` 的最小服务示例, 演示 `ext/http.Status` + `ext/slog`
- `examples/grpc`: gRPC 服务端示例, 演示业务码与 `status.Status` 的转换

#### 文档

- 新增 `README_CN.md` 中文版 README
- README 增补主流日志库 (logrus / zap / zerolog) 集成示例与对比说明

### Changed

- CI workflow 增强: 扩展矩阵, 加入对 `integration/*` 子模块的独立构建与测试

## [0.1.1] - 2026-06-08

### Changed

- 调整 `errkit` 包顶层 doc comment, 更清晰地表达项目定位

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

- `errors.Is(err, kind)` **不支持** (`*Kind` 不实现 `error`); 请用 `kind.Is(err)` 或 `errkind.CodeOf(err)`
- 暂未提供错误码冲突的静态检查工具 (规划在 v0.x)
- 暂未提供 i18n / metrics 自动发射 (规划在 v0.x)

[Unreleased]: https://github.com/im-wmkong/errkind/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/im-wmkong/errkind/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/im-wmkong/errkind/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/im-wmkong/errkind/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/im-wmkong/errkind/releases/tag/v0.1.0
