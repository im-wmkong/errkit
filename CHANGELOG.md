# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 与 [SemVer](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added

#### tooling (错误码工程化工具链)

- `cmd/errkindlint`: 静态扫描 `errkind.Define` 调用, 在编译期 (而非 init 期) 检出重复 code / 重复 name / 空 name; 发现冲突退出码非零, 可直接接入 CI
  - 支持 `-exclude=glob` 过滤 (例: `-exclude=examples/`)
  - 支持 Go 风格的 `./...` 路径
- `cmd/errkind doc`: 基于源码静态分析生成错误码文档, 跨团队对齐错误码契约
  - `-format=md` 输出 Markdown 表格 (含 code / name / 默认消息 / 源位置)
  - `-format=json` 输出结构化 JSON, 可喂给前端 codegen / SRE 告警平台
  - `-o=file` 写入文件, 默认 stdout
- `internal/scan`: 共享的 AST 扫描层, lint 与 doc 复用同一份解析结果, 仅识别字面量参数, 跳过 `_test.go` / `vendor` / `testdata`; 基于目录递归, 天然跨 `go.mod` 边界, 一次扫到主模块 + `integration/*` 全部源码

#### integration/grpc (流式与稳定性增强)

- 新增 `StreamServerInterceptor()` / `StreamClientInterceptor()`, 支持 server-streaming / client-streaming / bidi: 服务端 handler 返回的 errkind 错误自动 `ToStatus`, 客户端 `RecvMsg` / `SendMsg` / `CloseSend` 拿到的 status 自动 `FromStatus`
- `ToStatus` 把 attrs 插入序编码进 `_errkind.order` metadata, `FromStatus` 按此还原, 解决 `map[string]string` 序列化丢序问题; 客户端无 order 字段时按 key 字典序兜底, 保证跨调用稳定
- `*remoteErr` 实现 `GRPCStatus()`, 让 `status.FromError(err)` 能从客户端还原出的错误反向取回原 `*status.Status`, 不破坏 grpc 互操作约定

#### examples

- `examples/grpc` 升级为基于 `bufconn` 的 in-process server + client 完整闭环: 注册服务端 / 客户端 unary 拦截器, 通过真实 gRPC 调用演示业务码 / Kind name / attrs / grpc code 跨进程透传; 客户端用 `grpcint.IsReason` / `grpcint.CodeOf` / `grpcext.CodeOf` 做分支判断

#### CI / 工程

- workflow 新增 `errkindlint` 自检步骤; 覆盖整仓 (跨 `go.mod`), 防止重复错误码流入主干
- 覆盖率 gate 排除 `cmd/` 目录 (CLI 入口无业务逻辑)
- 新增 `scripts/test.sh`: 遍历仓内全部 `go.mod`, 串行跑 `go vet` / `go build` / `go test`, 让本地一行命令复刻 CI 的多 module 行为 (兼容 macOS 自带 bash 3.2)
- `scripts/test.sh` 增加 `--group` 选项, 在每个 module 前后发射 `::group::` / `::endgroup::`; CI workflow 的 vet / build / test 三段循环合并为一行 `./scripts/test.sh --group -race`, 与本地行为完全一致, 单一事实源, 避免双份维护漂移
- `examples/grpc` 拆分为 `server.go` / `client.go` / `main.go` 三文件, 主入口仅做协调, 业务/拨号逻辑各归其位

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
- ~~暂未提供错误码冲突的静态检查工具 (规划在 v0.x)~~ — 已在 Unreleased 提供 `cmd/errkindlint`
- 暂未提供 i18n / metrics 自动发射 (规划在 v0.x)

[Unreleased]: https://github.com/im-wmkong/errkind/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/im-wmkong/errkind/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/im-wmkong/errkind/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/im-wmkong/errkind/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/im-wmkong/errkind/releases/tag/v0.1.0
