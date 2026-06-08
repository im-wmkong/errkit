package errkit

import (
	"fmt"
	"sync"
)

// Registry 持有一组 Kind, 保证 (code, name) 在自身范围内唯一。
//
// 通常使用包级 Define / Kinds / LookupCode / LookupName 即可;
// 测试或多租户场景可以 NewRegistry() 创建独立注册中心。
type Registry struct {
	mu     sync.RWMutex
	byCode map[Code]*Kind
	byName map[string]*Kind
	all    []*Kind
}

// NewRegistry 创建一个独立的注册中心。
func NewRegistry() *Registry {
	return &Registry{
		byCode: map[Code]*Kind{},
		byName: map[string]*Kind{},
	}
}

// Define 注册并返回一个新的 Kind; 重复 code/name 立即 panic。
func (r *Registry) Define(code Code, name string, opts ...KindOption) *Kind {
	if name == "" {
		panic("errkit: Define name must not be empty")
	}
	k := &Kind{code: code, name: name}
	for _, o := range opts {
		o(k)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existed, ok := r.byCode[code]; ok {
		panic(fmt.Sprintf("errkit: duplicate code %d (registered as %q, new %q)",
			code, existed.name, name))
	}
	if existed, ok := r.byName[name]; ok {
		panic(fmt.Sprintf("errkit: duplicate name %q (registered with code %d, new %d)",
			name, existed.code, code))
	}
	r.byCode[code] = k
	r.byName[name] = k
	r.all = append(r.all, k)
	return k
}

// Kinds 返回所有已注册 Kind, 按注册顺序; 用于生成错误码文档。
func (r *Registry) Kinds() []*Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Kind, len(r.all))
	copy(out, r.all)
	return out
}

// LookupCode 按 code 查找; 不存在返回 nil。
func (r *Registry) LookupCode(c Code) *Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byCode[c]
}

// LookupName 按 name 查找; 不存在返回 nil。
func (r *Registry) LookupName(n string) *Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byName[n]
}

// KindOption 用于 Define 时配置 Kind 的默认行为。
type KindOption func(*Kind)

// DefaultMessage 给 Kind 设置默认消息, New / Wrap 未传 Message 时回退。
func DefaultMessage(msg string) KindOption {
	return func(k *Kind) { k.defaultMessage = msg }
}

// 包级默认 Registry。
var defaultRegistry = NewRegistry()

// Define 在默认注册中心注册一个 Kind。
func Define(code Code, name string, opts ...KindOption) *Kind {
	return defaultRegistry.Define(code, name, opts...)
}

// Kinds 返回默认注册中心的所有 Kind。
func Kinds() []*Kind { return defaultRegistry.Kinds() }

// LookupCode 默认注册中心查询。
func LookupCode(c Code) *Kind { return defaultRegistry.LookupCode(c) }

// LookupName 默认注册中心查询。
func LookupName(n string) *Kind { return defaultRegistry.LookupName(n) }
