module github.com/im-wmkong/errkind/examples/grpc

go 1.23

toolchain go1.23.10

require (
	github.com/im-wmkong/errkind v0.0.0
	github.com/im-wmkong/errkind/integration/grpc v0.0.0-20260609120453-e15168aec72f
	google.golang.org/grpc v1.66.0
	google.golang.org/protobuf v1.34.2
)

require (
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
)

replace github.com/im-wmkong/errkind => ../..
