module github.com/im-wmkong/errkit/integration/zap

go 1.23

toolchain go1.23.10

require (
	github.com/im-wmkong/errkit v0.0.0
	go.uber.org/zap v1.27.0
)

require go.uber.org/multierr v1.10.0 // indirect

replace github.com/im-wmkong/errkit => ../..
