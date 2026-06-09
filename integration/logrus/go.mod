module github.com/im-wmkong/errkit/integration/logrus

go 1.23

toolchain go1.23.10

require (
	github.com/im-wmkong/errkit v0.0.0
	github.com/sirupsen/logrus v1.9.3
)

require golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect

replace github.com/im-wmkong/errkit => ../..
