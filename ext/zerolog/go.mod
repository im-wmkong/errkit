module github.com/im-wmkong/errkit/ext/zerolog

go 1.23

toolchain go1.23.10

require (
	github.com/im-wmkong/errkit v0.0.0
	github.com/rs/zerolog v1.33.0
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	golang.org/x/sys v0.12.0 // indirect
)

replace github.com/im-wmkong/errkit => ../..
