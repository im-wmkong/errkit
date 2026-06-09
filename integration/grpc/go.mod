module github.com/im-wmkong/errkit/integration/grpc

go 1.23

toolchain go1.23.10

require (
	github.com/im-wmkong/errkit v0.0.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1
	google.golang.org/grpc v1.66.0
)

require (
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

replace github.com/im-wmkong/errkit => ../..
