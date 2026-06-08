package errkit

import "errors"

// 本文件提供从 error 链中"读"信息的 helper。
// 全部基于 errors.As, 不引入新机制。

// KindOf 从 err 链上提取第一个 errkit 错误的 Kind。
//
// 注意: 当 err 不是 errkit 错误时返回 nil。
// 推荐使用 CodeOf / NameOf 这两个 (T, bool) 风格的 helper, 避免空指针解引用:
//
//	if c, ok := errkit.CodeOf(err); ok && c == UserNotFound.Code() { ... }
//
// 或者显式 nil 判:
//
//	if k := errkit.KindOf(err); k != nil && k == UserNotFound { ... }
func KindOf(err error) *Kind {
	var e *kerr
	if errors.As(err, &e) {
		return e.kind
	}
	return nil
}

// CodeOf 返回错误的业务 Code 与是否找到; 是 KindOf 的 nil-safe 版本。
func CodeOf(err error) (Code, bool) {
	var e *kerr
	if errors.As(err, &e) {
		return e.kind.code, true
	}
	return 0, false
}

// NameOf 返回错误的 Kind name 与是否找到; 是 KindOf 的 nil-safe 版本。
func NameOf(err error) (string, bool) {
	var e *kerr
	if errors.As(err, &e) {
		return e.kind.name, true
	}
	return "", false
}

// MessageOf 返回错误的 Message; 没有 errkit 错误时返回 err.Error()。
func MessageOf(err error) string {
	var e *kerr
	if errors.As(err, &e) {
		return e.message
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

// AttrsOf 返回最外层 errkit 错误的 attrs 拷贝; 没有则返回 nil。
//
// 返回的是浅拷贝, 调用方可以安全修改, 不会影响原错误。
func AttrsOf(err error) []Attr {
	var e *kerr
	if errors.As(err, &e) {
		if len(e.attrs) == 0 {
			return nil
		}
		out := make([]Attr, len(e.attrs))
		copy(out, e.attrs)
		return out
	}
	return nil
}

// AllAttrs 沿错误链收集所有 errkit 错误的 attrs (扁平合并)。
//
// 同名 key 以"最外层"为准 (符合"内层是细节, 外层是上下文增强"的直觉);
// 99% 的日志场景需要这个扁平视图, 默认就给, 不让业务自己 Walk。
func AllAttrs(err error) []Attr {
	var out []Attr
	seen := map[string]struct{}{}
	for cur := err; cur != nil; {
		if e, ok := cur.(*kerr); ok {
			for _, a := range e.attrs {
				if _, exists := seen[a.Key]; !exists {
					seen[a.Key] = struct{}{}
					out = append(out, a)
				}
			}
		}
		u, ok := cur.(interface{ Unwrap() error })
		if !ok {
			break
		}
		cur = u.Unwrap()
	}
	return out
}
