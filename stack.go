package errkit

import (
	"runtime"
	"strconv"
	"sync/atomic"
)

// captureStack 控制是否在 New / Wrap 时抓取调用栈。
//
// 默认关闭。开发/Staging 环境通常 SetCaptureStack(true), 生产按需。
// 用进程级开关而不是 Option, 是因为"忘记加 WithStack" 是大概率事件。
var captureStack atomic.Bool

// SetCaptureStack 设置是否抓栈; 通常在 main 里调用一次。
func SetCaptureStack(on bool) { captureStack.Store(on) }

// Frame 是调用栈一帧。
type Frame struct {
	Function string
	File     string
	Line     int
}

// String 输出 "package.Func\n\tfile:line"。
func (f Frame) String() string {
	return f.Function + "\n\t" + f.File + ":" + strconv.Itoa(f.Line)
}

// Tracer 表示能提供调用栈的错误。
type Tracer interface {
	StackTrace() []Frame
}

// capturePCs 抓取当前调用栈的 PC 列表; skip 表示需要跳过的栈帧数 (含 capturePCs 自身)。
func capturePCs(skip int) []uintptr {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	if n == 0 {
		return nil
	}
	out := make([]uintptr, n)
	copy(out, pcs[:n])
	return out
}

// resolveFrames 把 PC 列表解析成可读 Frame; 用延迟解析是因为大多数错误从不被打印。
func resolveFrames(pcs []uintptr) []Frame {
	if len(pcs) == 0 {
		return nil
	}
	frames := runtime.CallersFrames(pcs)
	out := make([]Frame, 0, len(pcs))
	for {
		f, more := frames.Next()
		out = append(out, Frame{Function: f.Function, File: f.File, Line: f.Line})
		if !more {
			break
		}
	}
	return out
}

// hasStack 判断错误链上是否已经存在 Tracer, 避免 Wrap 时重复抓栈。
func hasStack(err error) bool {
	for err != nil {
		if _, ok := err.(Tracer); ok {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
