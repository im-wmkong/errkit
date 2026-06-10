package errkind

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// kerr 是 Kind.New / Kind.Wrap 产生的错误实例 (具体类型, 不导出)。
//
// 所有"作用于错误自身"的属性 (message / attrs / 调用栈) 都直接放这里;
// 而所有"协议相关"扩展 (HTTP status / gRPC code / ...) 不在这里,
// 由 ext 包用独立的装饰器 wrapper 包在外面。
type kerr struct {
	kind    *Kind
	message string
	attrs   []Attr
	cause   error
	pcs     []uintptr // 仅在抓栈开关打开时填充
}

// Error 输出 "user_not_found(10001): 用户不存在: <cause>"。
func (e *kerr) Error() string {
	var b strings.Builder
	b.WriteString(e.kind.name)
	b.WriteByte('(')
	b.WriteString(strconv.FormatUint(uint64(e.kind.code), 10))
	b.WriteByte(')')
	if e.message != "" {
		b.WriteString(": ")
		b.WriteString(e.message)
	}
	if e.cause != nil {
		b.WriteString(": ")
		b.WriteString(e.cause.Error())
	}
	return b.String()
}

func (e *kerr) Unwrap() error   { return e.cause }
func (e *kerr) Kind() *Kind     { return e.kind }
func (e *kerr) Message() string { return e.message }

// Attrs 返回 attrs 的浅拷贝; 调用方可安全修改, 不会影响原错误。
//
// 与 AttrsOf 的约定一致——对外暴露 attrs 时一律拷贝, 避免内部状态被篡改。
func (e *kerr) Attrs() []Attr {
	if len(e.attrs) == 0 {
		return nil
	}
	return append([]Attr(nil), e.attrs...)
}

// StackTrace 返回延迟解析后的调用栈; 没抓栈时返回 nil。
func (e *kerr) StackTrace() []Frame {
	return resolveFrames(e.pcs)
}

// Format 实现 fmt.Formatter:
//
//	%s / %v   -> 与 Error() 相同的单行格式
//	%q        -> 单行格式加引号
//	%+v       -> 多行: 错误链 + (若抓了栈) 调用栈
//
// 这是 Go error 库的事实标准, 方便 %+v 直接打出可读的诊断信息。
func (e *kerr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			io.WriteString(s, e.Error())
			if st := e.StackTrace(); len(st) > 0 {
				for _, f := range st {
					io.WriteString(s, "\n")
					io.WriteString(s, f.String())
				}
			}
			return
		}
		io.WriteString(s, e.Error())
	case 's':
		io.WriteString(s, e.Error())
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	}
}

// MarshalJSON 实现 json.Marshaler, 让 json.Marshal(err) 输出结构化字段:
//
//	{"code":10001,"name":"user_not_found","message":"用户不存在",
//	 "attrs":{"uid":42},"cause":"sql: no rows in result set"}
//
// 这是最通用的方案: 不含 HTTP/gRPC 等协议字段 (那些由 ext 层负责),
// 仅输出 core 的领域字段。cause 若也是 errkind 错误则嵌套展开, 否则取字符串。
//
// attrs 按插入顺序输出 (手写而非 map[string]any, 保证顺序稳定)。
func (e *kerr) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	buf.WriteString(`"code":`)
	buf.WriteString(strconv.FormatUint(uint64(e.kind.code), 10))

	buf.WriteString(`,"name":`)
	writeJSONString(&buf, e.kind.name)

	if e.message != "" {
		buf.WriteString(`,"message":`)
		writeJSONString(&buf, e.message)
	}

	if len(e.attrs) > 0 {
		buf.WriteString(`,"attrs":{`)
		for i, a := range e.attrs {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeJSONString(&buf, a.Key)
			buf.WriteByte(':')
			vRaw, err := json.Marshal(a.Val)
			if err != nil {
				// 不可序列化的值降级为字符串, 避免整条日志失败
				vRaw, _ = json.Marshal(fmt.Sprintf("%v", a.Val))
			}
			buf.Write(vRaw)
		}
		buf.WriteByte('}')
	}

	if e.cause != nil {
		buf.WriteString(`,"cause":`)
		if ce, ok := e.cause.(*kerr); ok {
			raw, err := ce.MarshalJSON()
			if err != nil {
				return nil, err
			}
			buf.Write(raw)
		} else if jm, ok := e.cause.(json.Marshaler); ok {
			raw, err := jm.MarshalJSON()
			if err != nil {
				return nil, err
			}
			buf.Write(raw)
		} else {
			writeJSONString(&buf, e.cause.Error())
		}
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// writeJSONString 把字符串安全编码进 buf (含转义)。
func writeJSONString(buf *bytes.Buffer, s string) {
	raw, _ := json.Marshal(s)
	buf.Write(raw)
}
